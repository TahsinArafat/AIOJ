package problemgen

import (
	"context"
	"fmt"
	"time"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/harness"
)

// SystemPrompt instructs the model to build a problem with the tool catalog.
//
// It states the two constraints the model cannot discover for itself: that it
// must never try to type a large test case (it would be truncated), and that
// finalize enforces a bar it cannot negotiate.
const SystemPrompt = `You are an expert competitive-programming problem setter.

You build one problem by calling tools. You cannot see file contents: tools
return metadata (size, line count, hash, first and last line) and verdicts.
This is deliberate — a test case with 100000 lines cannot be written by you and
must be produced by the generator program you write.

Work in this order:
1. set_constraints — the numeric bounds. Every case is validated against them.
2. set_limits — judge limits the reference solution must meet at n_max.
3. set_generator_shapes — the input shapes your generator supports.
4. write_generator — invoked as: ./gen --seed <uint64> --n <int> --shape <name>
   It must print exactly one test case to stdout, honour --n, and be
   deterministic for a given seed (seed your RNG from --seed, never from time).
5. write_solution — the reference solution. It must pass n_max within the limits.
6. write_brute — optional but strongly recommended; it is cross-checked against
   your reference on small cases, which is the only way a wrong reference is
   caught.
7. write_validator — optional; rejects inputs outside the declared constraints.
8. set_statement — title, statement, formats, samples. Samples are verified
   against the reference solution, so they must be correct.
9. materialize_matrix, then validate_cases, check_samples, check_determinism.
10. finalize — runs every gate. It fails if any gate fails; fix and retry.

Rules:
- Never attempt to emit a large input as text. Use generate_case or
  materialize_matrix and let the generator produce it.
- Fix compile errors from the reported log; do not rewrite blindly.
- Keep the statement consistent with the constraints you declared.
- Write mathematics as LaTeX wrapped in single dollar signs: $1 \le n \le 2\cdot10^5$,
  $a_i$, $O(n \log n)$. The renderer runs remark-math plus KaTeX, so this is the
  only syntax that becomes typeset math.
- Do NOT put mathematics in inline-code spans (backtick pairs). Those render as
  literal monospace text, so 10^9 reaches the reader as the four characters
  "10^9" instead of a typeset power, and a_i never becomes a subscript. Reserve
  inline code for tokens that must stay verbatim, such as a language keyword.
- Express memory limits in KiB, not decimal bytes: 256 MB is 262144, 512 MB is
  524288, 1 GB is 1048576. The problem page renders memory_limit_kb / 1024 as
  MB, so a decimal value like 256000 displays as "250 MB".`

// Pipeline composes the loop, the catalog and the workspace.
type Pipeline struct {
	Model      harness.Model
	Runner     Runner
	Corpus     Corpus
	Transcript harness.Transcript
	Budget     harness.Budget
	// StagingRoot is the parent directory for per-job case files.
	StagingRoot string
	// MaxBytes bounds materialized data per job.
	MaxBytes int64
}

// Run executes one generation job and returns the manifest.
//
// The manifest is what apply consumes: statement, constraints, limits, every
// case with its hash, and the artifact sources, so a problem can be
// regenerated and audited later.
func (p *Pipeline) Run(ctx context.Context, jobID string, brief string) (*Manifest, error) {
	if p.Model == nil {
		return nil, fmt.Errorf("problemgen: a model is required")
	}
	if p.Runner == nil {
		return nil, fmt.Errorf("problemgen: a runner is required")
	}
	if p.StagingRoot == "" {
		return nil, fmt.Errorf("problemgen: a staging root is required")
	}

	ws, err := NewWorkspace(WorkspaceOptions{
		Root:    p.StagingRoot,
		JobID:   jobID,
		Runner:  p.Runner,
		MaxByte: p.MaxBytes,
	})
	if err != nil {
		return nil, err
	}

	tools := NewTools(ws, p.Corpus)
	started := time.Now()
	outcome, err := (&harness.Loop{
		Model:      p.Model,
		Tools:      tools,
		Transcript: p.Transcript,
		Budget:     p.Budget,
		System:     SystemPrompt,
	}).Run(ctx, jobID, []ai.Message{{Role: ai.RoleUser, Content: brief}})
	if err != nil {
		return nil, err
	}

	m := &Manifest{
		Constraints: ws.Constraints(),
		Limits:      ws.Limits(),
		Statement:   tools.statement,
		Cases:       ws.Cases(),
		Artifacts:   map[string]string{},
		Languages:   map[string]string{},
		Usage:       ws.Usage(),
	}
	for _, kind := range AllArtifacts {
		if src, ok := ws.ArtifactSource(kind); ok {
			m.Artifacts[string(kind)] = src
			m.Languages[string(kind)] = ws.ArtifactLanguage(kind)
		}
	}
	m.Usage.SandboxCPU = outcome.Usage.SandboxCPU
	if m.Usage.SandboxCPU == 0 {
		m.Usage.SandboxCPU = time.Since(started)
	}
	m.Usage.BytesWritten = outcome.Usage.BytesWritten
	m.Usage.Turns = outcome.Usage.Turns

	// A run that stopped on a budget did not finish; report it as not ready
	// rather than letting a partial manifest reach review as if it were whole.
	if !outcome.Completed() {
		m.Report = Report{
			Ready:   false,
			Checks:  []Check{{Name: "harness", Passed: false, Message: "the run stopped early: " + outcome.StopReason}},
			Cases:   m.Cases,
			Blocked: []string{"harness"},
		}
		return m, nil
	}

	// Re-run the gates outside the loop so a model that simply never called
	// finalize cannot produce an unvalidated problem.
	report := Report{Ready: true}
	report.Checks = append(report.Checks, ws.CheckCoverage()...)
	if largest, ok := ws.largestCase(); ok {
		report.Checks = append(report.Checks, ws.CheckDeterminism(ctx, largest))
	}
	report.Checks = append(report.Checks, ws.CheckSamples(ctx, tools.statement.Samples)...)
	report.Checks = append(report.Checks, ws.CheckLimits(ctx)...)
	report.Cases = m.Cases
	for _, c := range report.Checks {
		if !c.Passed {
			report.Ready = false
		}
	}
	report.Blocked = report.Failed()
	m.Report = report

	return m, nil
}
