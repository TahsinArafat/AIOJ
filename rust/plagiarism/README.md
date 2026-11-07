# aioj-plagiarism

The pairwise source-similarity engine behind AIOJ's plagiarism reports.

This is the one part of the backend that is CPU-bound rather than I/O-bound, so it
is written in Rust and runs as a short-lived subprocess. Everything else — report
lifecycle, SQL, thresholds, what counts as a pair — stays in Go.

## Why this component and not the rest

A plagiarism check compares every eligible pair of accepted submissions on a
problem: `n(n-1)/2` comparisons, each an LCS over token streams. At the service's
cap of 300 submissions that is 44,850 pairs, and for a typical ~900-token
solution the earlier Go implementation needed **~96 seconds per problem** while
allocating a fresh 6.7 MB dynamic-programming matrix for every comparison.

Measured on the same 44,850-pair corpus (Apple M4, 10 cores, 900-token sources):

| Implementation | Time | Note |
|---|---|---|
| Original Go loop (`CompareCodes` per pair) | ~96 s | full matrix per pair, re-tokenized per pair |
| `NativeEngine` (Go, this repo) | 17.0 s | tokenize once, intersection prefilter, rolling rows |
| `aioj-plagiarism` (Rust) | **0.68 s** | + bit-parallel LCS, all cores |

On *short* sources (≈90 tokens) the two engines are indistinguishable: the fixed
cost of the process boundary is the same order as the whole scan. The Rust engine
earns its place when the LCS work is real, which is precisely the case that made
the old implementation miss its own 10-minute timeout.

## How it runs

```
POST /api/contests/:id/plagiarism
  └── Go: plagiarism.Service.RunCheck
        ├── collect accepted submissions (capped at 300/problem)
        ├── exec aioj-plagiarism   ── JSON in, NDJSON out, one process per report
        └── write plagiarism_pairs, update report counts
```

It is a subprocess rather than cgo or a resident service because:

- no cgo keeps the backend on `CGO_ENABLED=0` and keeps a Rust toolchain out of
  the Go build;
- a resident service would mean a new port and a new unauthenticated surface for
  no benefit — a scan is bounded and short-lived;
- a crash is an error the service can fall back from, not a corrupted report.

If the binary is missing or fails, the Go service logs it and re-runs the scan on
`NativeEngine`, so a broken deployment degrades in speed, never in correctness.

## Protocol

One JSON request on stdin. Source text is escaped, so the request is a single
line even though the code is not:

```json
{"threshold":0.7,"inputs":[
  {"key":"sub-1","problem_id":"prob-1","user_id":"user-1","code":"int main(){...}"}
]}
```

The response is newline-delimited JSON — one object per flagged pair, then
exactly one summary:

```json
{"pair":{"problem_id":"prob-1","a":"sub-1","b":"sub-2","similarity":0.83,"matched_tokens":742}}
{"summary":{"inputs":300,"pairs_total":44850,"pairs_scored":1512,"flagged":37,"engine":"rust","elapsed_ms":684}}
```

Errors are also JSON on stdout (`{"error":"..."}`) with a non-zero exit code, so
a caller always receives a parseable stream and can distinguish "the engine
refused" from "the engine is not the program we expected".

`--tokens` reads raw source on stdin and prints the space-joined token stream. It
exists for one reason: to let the Go test suite prove the two tokenizers agree
instead of assuming it.

## Exactness, not approximation

Three claims are tested rather than asserted:

1. **The LCS is exact.** `lcs_len` is checked against a textbook dynamic program
   on 2,000 random inputs and across the 63/64/65/127/128 word boundaries where a
   bit-parallel implementation leaks bits (`src/lcs.rs`).
2. **The prefilter is sound.** Any common subsequence is a common multiset, so
   `LCS <= multiset intersection`; a pair whose bound is below the threshold
   cannot cross it. `bound_is_never_below_the_true_similarity` checks the bound
   never under-estimates on random pairs.
3. **The two engines agree with the original Go reference.** A corpus of
   near-threshold copies, renamed variables, comment/whitespace variants and
   non-ASCII source is scored by `CompareCodes` and by both engines at ten
   thresholds; pair sets, similarity values and the report's pair count must
   match exactly (`internal/plagiarism/engine_parity_test.go`).

The tokenizer deliberately reproduces the Go regex pipeline's *quirks*, including
`//` being stripped inside a string literal (`"http://x"` loses its tail) and an
unterminated `/*` being kept. A "better" tokenizer would change every score.

## Building and testing

```bash
make rust-build     # cargo build --release, then copy to bin/aioj-plagiarism
make rust-test      # 34 unit tests
make rust-lint      # clippy, warnings as errors
make rust-fmt       # rustfmt --check
make test-parity    # builds the engine, then runs the Go differential tests
```

Without `make rust-build` nothing breaks: `go test ./internal/plagiarism/` skips
the Rust cases and the backend uses the pure-Go engine.

## Selecting the engine

| `PLAGIARISM_ENGINE` | Behaviour |
|---|---|
| `auto` (default) | Rust when the binary is found, Go otherwise; logs which |
| `native` | Always Go |
| `rust` | Rust, and **startup fails** if the binary is missing |

`PLAGIARISM_RUST_BIN` points at the executable. Search order: that variable,
`./bin`, `<module>/bin`, `rust/plagiarism/target/{release,debug}`, then `PATH`.
The container image sets it to `/app/bin/aioj-plagiarism`.

Asking for `rust` explicitly fails loudly rather than downgrading: an operator who
set it wants to know it is not in effect.
