# AI problem generation: the harness pipeline

The generator is a **multi-turn tool loop**, not a single prompt. The model
drafts, compiles, generates test data, runs it, reads verdicts, and fixes what
failed — the way a human setter works.

The one constraint that shapes everything: **a model response cannot contain a
test case.** `max_output_tokens` defaults to 8192, so a 1e5-line input is
physically impossible to emit as text. Every large case is therefore produced by
*executing* a generator program inside go-judge.

## Architecture

```
cmd/ai-harness  (own container, --mode=ai-harness)
  │  harness.Loop — bounded, resumable tool conversation
  │  problemgen.Pipeline — workspace + catalog + gates
  ▼
problemgen.Runner  ──►  go-judge   (the only place generated code runs)
```

Two seams keep this swappable:

- **`harness.ToolExecutor`** — the loop owns no tools. In-process and
  separate-container deployments use the same loop and the same catalog.
- **`problemgen.Runner`** — everything that executes code goes through it, so
  the tools are testable without a live judge and a different sandbox can be
  substituted without touching the catalog.

The model chooses *what to do next*. It never chooses *what gets executed*: there
is no shell tool, no tool that accepts a path, and no language outside the
`lang/*.yaml` registry.

## Decision order, and who can change the bar

| Layer | Where | Who controls it |
|---|---|---|
| Which tool to call next | model | model |
| Tool arguments | model | validated server-side |
| Case matrix (tiers, sizes) | `problemgen.Matrix` | **pipeline** |
| Gate thresholds | `finalize` | **pipeline** |
| Publish | `ready → review → apply` | human |

A run that never calls `finalize`, or stops on a budget, is stored as **not
ready**. The pipeline also re-runs the gates after the loop returns, so a model
cannot skip validation by simply not asking for it.

## Tools

**Authoring.** `set_constraints`, `set_limits`, `set_generator_shapes`,
`set_statement`, `write_generator`, `write_solution`, `write_brute`,
`write_validator`.

**Materialization.** `generate_case` (seed, n, shape),
`materialize_matrix`, `validate_cases`.

**Verification.** `run_reference`, `check_samples`, `check_determinism`,
`search_problems`, `finalize`.

Execution tools return **metadata only** — bytes, line count, SHA-256, first and
last line. A 1 MB case never enters the context window.

### Generator contract

```
./gen --seed <uint64> --n <int> --shape <name>
```

Flags rather than positional arguments, deliberately: the program is
model-written, and a silently misparsed positional argument produces a plausible
wrong case, while a bad flag is a hard parse error.

## The gates

`finalize` refuses to produce `ready` unless all of these pass:

| Gate | Catches |
|---|---|
| `coverage` | missing max/min tier; a generator that ignores its arguments (identical bytes across different `(n, shape)` cells) |
| `determinism` | RNG seeded from the clock or unseeded `rand()` — the most common defect in generated problem code |
| `validation` | `n > n_max`, values out of range, non-integer tokens; plus the problem's own validator if one was written |
| `reference` | reference solution failing on a case |
| `samples` | statement examples the reference solution cannot reproduce |
| `limits` | reference solution not passing `n_max` inside the declared limits — the problem is unsolvable as written |
| `crosscheck` | brute force disagreeing with the reference on small cases — the only check that catches a *wrong* reference solution |

`limits_headroom` additionally warns when the reference uses ≥60 % of the time
limit, which would TLE on a slower judge.

## Budgets

Exhausting any dimension stops the run with a named reason instead of spinning:

| Budget | Default |
|---|---|
| `max_turns` | 10 |
| `max_tool_calls` | 60 |
| `max_total_tokens` | 200 000 |
| `max_wall_clock` | 5 min |
| `max_sandbox_cpu` | 120 s |
| `max_bytes` | 64 MiB |

Plus `AI_MAX_CASE_N` (default 1e6): the judge loads a whole input into memory
and ships it as sandbox `CopyIn`, so an unbounded `n` is a host-memory risk, not
only a disk one.

## Resume and replay

Turns are persisted in `ai_harness_turns`. Two consequences:

- A worker that dies mid-run **continues at the next turn** instead of paying for
  the whole conversation again. This is the specific thing a fixed retry loop
  gets wrong: same prompt → same failure → full cost, forever.
- A run is **replayable**: every tool call and result is stored, so it can be
  reconstructed for audit. (A free-form agent transcript cannot offer this.)

A job whose heartbeat goes stale for 10 minutes is requeued, so a killed
container does not strand work.

## Configuration

| Env | Default | Meaning |
|---|---|---|
| `AI_ENABLED` | `false` | master switch for all AI features |
| `AI_HARNESS_ENABLED` | `false` | enables `--mode=ai-harness` |
| `AI_CREDENTIAL_MASTER_KEY` | — | required; 32-byte base64, encrypts provider keys |
| `AI_MAX_CASE_N` | `1000000` | largest permitted `n` |
| `AI_MAX_JOB_BYTES` | `67108864` | materialized data per job |
| `AI_REPAIR_TURNS` | `2` | reserved for the bounded repair loop |
| `AI_STAGED_TESTDATA_ROOT` | `./testdata/ai-staging` | where cases are materialized |

Run it as its own service:

```bash
docker compose --profile ai up -d ai-harness
```

## Adding a tool

1. Register it in `problemgen.Tools()` with a closed JSON schema
   (`additionalProperties: false`, explicit `required`).
2. Add a case in `Tools.Execute`, validate every argument, and return
   `harness.Result` with `Content` kept small.
3. If it executes code, go through `Runner` — never `os/exec`.
4. Add a test asserting the failure mode it is meant to catch.

`TestCatalogExposesNoShellTool` enforces the security invariant for the whole
catalog: every tool must have a closed schema, and no tool may be named
`exec`, `run_shell`, `read_file`, `write_file` or similar.
