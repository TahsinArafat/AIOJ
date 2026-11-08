package problemgen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/harness"
)

// SimilarProblem is one corpus hit from search_problems.
type SimilarProblem struct {
	Slug       string  `json:"slug"`
	Title      string  `json:"title"`
	Similarity float64 `json:"similarity"`
}

// Corpus answers "has this been set before?".
//
// An interface rather than a store dependency so the tool catalog stays
// testable and so a deployment can point it at the local tsvector index, an
// external judge, or nothing at all.
type Corpus interface {
	Similar(ctx context.Context, title, description string, limit int) ([]SimilarProblem, error)
}

// Tools is the problem-generation tool catalog.
//
// Every tool is named, schema-validated and bounded. There is deliberately no
// "run arbitrary code" tool and no tool that accepts a filesystem path: the
// model chooses what to do next, never what gets executed or where it lands.
type Tools struct {
	ws     *Workspace
	corpus Corpus
	// shapes and samples arrive through tools, so the catalog holds the
	// authoring state that is not itself a file.
	statement    Statement
	shapes       []string
	seedsPerCell int
	// noveltyLimit is the similarity above which a match is reported as a
	// likely duplicate.
	noveltyLimit float64
}

// NewTools builds a catalog over a workspace.
func NewTools(ws *Workspace, corpus Corpus) *Tools {
	return &Tools{
		ws:           ws,
		corpus:       corpus,
		seedsPerCell: 1,
		noveltyLimit: 0.85,
	}
}

// Tools returns the catalog offered to the model.
func (t *Tools) Tools() []ai.Tool {
	return []ai.Tool{
		ai.NewTool("set_constraints",
			"Declare the problem's numeric bounds. The case matrix is sized from n_max and every generated case is validated against these values, so they must match the statement.",
			object(map[string]any{
				"n_min":     integer("Smallest n the problem allows (>= 1)."),
				"n_max":     integer("Largest n the problem allows."),
				"sum_n_max": integer("Optional bound on the sum of n over test cases."),
				"value_min": integer("Smallest value appearing in the input."),
				"value_max": integer("Largest value appearing in the input."),
			}, "n_min", "n_max", "value_min", "value_max")),

		ai.NewTool("set_limits",
			"Declare the judge limits. The reference solution must pass the maximum case inside these limits. Use KiB: 256 MB is 262144, 512 MB is 524288.",
			object(map[string]any{
				"time_limit_ms":   integer("Per-case CPU limit in milliseconds."),
				"memory_limit_kb": integer("Per-case memory limit in KiB. Use 262144 for 256 MB, 524288 for 512 MB, 1048576 for 1 GB; a decimal value like 256000 displays as 250 MB."),
			}, "time_limit_ms", "memory_limit_kb")),

		ai.NewTool("set_statement",
			"Write the problem statement. Samples are verified against the reference solution, so they must be correct. Write mathematics as $...$ LaTeX (KaTeX renders it); backticks render as literal monospace text, never as math. IMPORTANT: (1) Put the numeric bounds in the dedicated `constraints` field as a LaTeX block — it renders as its own Constraints section; do NOT repeat them in the description. (2) Leave hint empty — it renders as a Notes section visible to contestants and must NOT contain the solution approach or algorithm.",
			object(map[string]any{
				"title":         str("Short problem title."),
				"description":   str("Full Markdown statement (scenario and task only). Mathematical notation must be $...$ LaTeX. Do NOT put the numeric bounds here — use the constraints field. Do NOT reveal the solution algorithm or key insight."),
				"constraints":   str("The numeric bounds as a LaTeX block, one bound per line, e.g. \"$1 \\\\le n \\\\le 2 \\\\cdot 10^5$\\n\\n$1 \\\\le a_i \\\\le 10^7$\". Every bound declared in set_constraints must appear here."),
				"input_format":  str("Input format description."),
				"output_format": str("Output format description."),
				"hint":          str("Leave EMPTY. The hint field renders as a Notes section visible to contestants. Do not put the solution approach, algorithm, or editorial content here."),
				"tags":          strings_("Algorithmic topics."),
				"difficulty":    str("One of easy, medium, hard."),
				"samples": arrayOf(object(map[string]any{
					"input":       str("Sample input."),
					"output":      str("Expected output."),
					"explanation": str("Optional: explain why the output is correct mechanically (LaTeX allowed). Do NOT reveal the general algorithm or approach."),
				}, "input", "output")),
			}, "title", "description", "constraints", "input_format", "output_format", "tags", "difficulty", "samples")),

		ai.NewTool("set_generator_shapes",
			"Declare the input shapes the generator supports (for example random, sorted, reverse, star, line). The case matrix runs every tier against every shape.",
			object(map[string]any{
				"shapes": arrayOf(str("A shape name the generator accepts via --shape.")),
			}, "shapes")),

		ai.NewTool("write_generator",
			"Write and compile the test-case generator. It is invoked as `./gen --seed <uint64> --n <int> --shape <name>` and must write exactly one test case to stdout. It must be deterministic for a given seed and must respect --n.",
			object(map[string]any{
				"language": str("Language id from the judge registry, e.g. cpp-gpp-64."),
				"source":   str("Complete program source."),
			}, "language", "source")),

		ai.NewTool("write_solution",
			"Write and compile the reference solution. It must solve the problem within the declared limits at n_max.",
			object(map[string]any{
				"language": str("Language id."),
				"source":   str("Complete program source."),
			}, "language", "source")),

		ai.NewTool("write_brute",
			"Optional: write a brute-force solution. If present it is cross-checked against the reference solution on small cases, which is the only way to catch a wrong reference solution.",
			object(map[string]any{
				"language": str("Language id."),
				"source":   str("Complete program source."),
			}, "language", "source")),

		ai.NewTool("write_validator",
			"Optional but recommended: write an input validator. It reads a test case on stdin and exits non-zero if the input is malformed or outside the declared constraints.",
			object(map[string]any{
				"language": str("Language id."),
				"source":   str("Complete program source."),
			}, "language", "source")),

		ai.NewTool("generate_case",
			"Run the generator and materialize one test case. Returns metadata only (bytes, line count, hash, first and last line) — never the file contents.",
			object(map[string]any{
				"seed":  integer("Seed passed to the generator."),
				"n":     integer("Value of n to request."),
				"shape": str("Shape name from set_generator_shapes."),
			}, "seed", "n", "shape")),

		ai.NewTool("materialize_matrix",
			"Materialize the standard case matrix derived from the constraints: min, small, medium, max and pathological tiers across every declared shape.",
			object(map[string]any{
				"seeds_per_cell": integer("Optional seeds per (tier, shape) cell; defaults to 1."),
			})),

		ai.NewTool("validate_cases",
			"Validate every materialized case against the declared constraints and, if a validator is present, run it. Reports violations.",
			object(map[string]any{})),

		ai.NewTool("run_reference",
			"Run the reference solution on a materialized case and derive its expected output. Reports the verdict, time and memory.",
			object(map[string]any{
				"case_name": str("Case name returned by generate_case or materialize_matrix."),
			}, "case_name")),

		ai.NewTool("check_samples",
			"Verify that the reference solution reproduces every statement sample.",
			object(map[string]any{})),

		ai.NewTool("check_determinism",
			"Verify the generator repeats itself for a case's seed. Run this on at least one case before finalizing.",
			object(map[string]any{
				"case_name": str("Case name to check."),
			}, "case_name")),

		ai.NewTool("search_problems",
			"Search the existing problem corpus for a similar problem. Use it before finalizing to avoid publishing a near-duplicate.",
			object(map[string]any{
				"title":       str("Proposed title."),
				"description": str("Proposed statement (may be abbreviated)."),
				"limit":       integer("Maximum number of results."),
			}, "title")),

		ai.NewTool("finalize",
			"Run every gate (coverage, determinism, validation, samples, limits) and mark the problem ready for human review. Returns the report.",
			object(map[string]any{})),
	}
}

// Execute dispatches one tool call.
func (t *Tools) Execute(ctx context.Context, call ai.ToolCall) (harness.Result, error) {
	args, err := decodeArgs(call.Function.Arguments)
	if err != nil {
		return harness.Result{}, err
	}

	switch call.Function.Name {
	case "set_constraints":
		return t.setConstraints(args)
	case "set_limits":
		return t.setLimits(args)
	case "set_statement":
		return t.setStatement(args)
	case "set_generator_shapes":
		return t.setShapes(args)
	case "write_generator":
		return t.writeArtifact(ctx, ArtifactGenerator, args)
	case "write_solution":
		return t.writeArtifact(ctx, ArtifactSolution, args)
	case "write_brute":
		return t.writeArtifact(ctx, ArtifactBrute, args)
	case "write_validator":
		return t.writeArtifact(ctx, ArtifactValidator, args)
	case "generate_case":
		return t.generateCase(ctx, args)
	case "materialize_matrix":
		return t.materializeMatrix(ctx, args)
	case "validate_cases":
		return t.validateCases(ctx)
	case "run_reference":
		return t.runReference(ctx, args)
	case "check_samples":
		return t.checkSamples(ctx)
	case "check_determinism":
		return t.checkDeterminism(ctx, args)
	case "search_problems":
		return t.searchProblems(ctx, args)
	case "finalize":
		return t.finalize(ctx)
	default:
		return harness.Result{}, fmt.Errorf("unknown tool %q", call.Function.Name)
	}
}

// --- authoring ------------------------------------------------------------

func (t *Tools) setConstraints(args map[string]any) (harness.Result, error) {
	c := Constraints{
		NMin:     intArg(args, "n_min"),
		NMax:     intArg(args, "n_max"),
		SumNMax:  intArg(args, "sum_n_max"),
		ValueMin: int64(intArg(args, "value_min")),
		ValueMax: int64(intArg(args, "value_max")),
	}
	if err := t.ws.SetConstraints(c); err != nil {
		return harness.Result{}, err
	}
	return ok(map[string]any{"constraints": c, "max_case_n": c.NMax})
}

func (t *Tools) setLimits(args map[string]any) (harness.Result, error) {
	l := Limits{TimeLimitMS: intArg(args, "time_limit_ms"), MemoryLimitKB: intArg(args, "memory_limit_kb")}
	if err := t.ws.SetLimits(l); err != nil {
		return harness.Result{}, err
	}
	return ok(map[string]any{"limits": l})
}

// setStatement validates and stores the statement.
func (t *Tools) setStatement(args map[string]any) (harness.Result, error) {
	st := Statement{
		Title:        strArg(args, "title"),
		Description:  strArg(args, "description"),
		Constraints:  strArg(args, "constraints"),
		InputFormat:  strArg(args, "input_format"),
		OutputFormat: strArg(args, "output_format"),
		Hint:         strArg(args, "hint"),
		Tags:         strSlice(args, "tags"),
		Difficulty:   strArg(args, "difficulty"),
	}
	if st.Title == "" || st.Description == "" {
		return harness.Result{}, fmt.Errorf("title and description are required")
	}
	switch st.Difficulty {
	case "easy", "medium", "hard", "":
	default:
		return harness.Result{}, fmt.Errorf("difficulty must be easy, medium or hard")
	}

	// Constraints live in their own field and render as a dedicated section. A
	// statement without declared bounds is incomplete, so this is required.
	if strings.TrimSpace(st.Constraints) == "" {
		return harness.Result{}, fmt.Errorf(
			"the constraints field is required: list every numeric bound as LaTeX, one per line, e.g. " +
				"\"$1 \\le n \\le 2 \\cdot 10^5$\" and \"$1 \\le a_i \\le 10^7$\". " +
				"These must match the bounds you passed to set_constraints.")
	}

	// Warn if the hint is non-empty — it renders as a Notes section visible to
	// contestants and is a common place for accidental solution spoilers.
	warningMsg := ""
	if strings.TrimSpace(st.Hint) != "" {
		warningMsg = "WARNING: the hint field is not empty. It renders as a Notes section visible to contestants. " +
			"Remove any solution approach, algorithm, or key insight from the hint."
	}

	raw, _ := args["samples"].([]any)
	if len(raw) > 3 {
		return harness.Result{}, fmt.Errorf("at most 3 samples are allowed")
	}
	for _, r := range raw {
		m, _ := r.(map[string]any)
		s := Sample{Input: strArg(m, "input"), Output: strArg(m, "output"), Explanation: strArg(m, "explanation")}
		if s.Input == "" {
			return harness.Result{}, fmt.Errorf("every sample needs an input")
		}
		st.Samples = append(st.Samples, s)
	}
	t.statement = st
	result, _ := ok(map[string]any{"samples": len(st.Samples), "constraints_present": true})
	if warningMsg != "" {
		result.Content = mustJSON(map[string]any{"ok": true, "samples": len(st.Samples), "constraints_present": true, "warning": warningMsg})
	}
	return result, nil
}

func (t *Tools) setShapes(args map[string]any) (harness.Result, error) {
	shapes := strSlice(args, "shapes")
	if len(shapes) == 0 {
		return harness.Result{}, fmt.Errorf("at least one shape is required")
	}
	if len(shapes) > 6 {
		return harness.Result{}, fmt.Errorf("at most 6 shapes are supported")
	}
	t.shapes = shapes
	return ok(map[string]any{"shapes": shapes})
}

func (t *Tools) writeArtifact(ctx context.Context, kind Artifact, args map[string]any) (harness.Result, error) {
	language := strArg(args, "language")
	source := strArg(args, "source")
	out, err := t.ws.WriteArtifact(ctx, kind, language, source)
	if err != nil {
		return harness.Result{}, err
	}
	if !out.OK {
		// A compile failure is a normal, recoverable outcome: report the log so
		// the model can fix the code on the next turn.
		return harness.Result{}, fmt.Errorf("%s failed to compile:\n%s", kind, out.Output)
	}
	return ok(map[string]any{"artifact": kind, "compiled": true})
}

// --- materialization ------------------------------------------------------

func (t *Tools) generateCase(ctx context.Context, args map[string]any) (harness.Result, error) {
	entry := MatrixEntry{
		Tier:  Tier(strArg(args, "tier")),
		N:     intArg(args, "n"),
		Shape: strArg(args, "shape"),
		Seed:  int64(intArg(args, "seed")),
	}
	if entry.Tier == "" {
		entry.Tier = TierMedium
	}
	if entry.Shape == "" {
		entry.Shape = "random"
	}
	if entry.Seed == 0 {
		entry.Seed = 1
	}

	c, err := t.ws.Materialize(ctx, entry)
	if err != nil {
		return harness.Result{}, err
	}
	if err := t.ws.ValidateCase(c); err != nil {
		return harness.Result{}, err
	}
	return harness.Result{
		Content:           mustJSON(map[string]any{"case": caseView(c)}),
		BytesMaterialized: c.Bytes,
		SandboxCPU:        0,
	}, nil
}

func (t *Tools) materializeMatrix(ctx context.Context, args map[string]any) (harness.Result, error) {
	seeds := intArg(args, "seeds_per_cell")
	if seeds <= 0 {
		seeds = t.seedsPerCell
	}
	matrix := Matrix(t.ws.Constraints(), t.shapes, seeds)
	if len(matrix) == 0 {
		return harness.Result{}, fmt.Errorf("constraints and shapes must be set before materializing")
	}

	var made []map[string]any
	var bytes int64
	for _, entry := range matrix {
		c, err := t.ws.Materialize(ctx, entry)
		if err != nil {
			// One bad cell should not discard the cases that worked.
			return harness.Result{}, fmt.Errorf("cell %s/%s (n=%d): %w", entry.Tier, entry.Shape, entry.N, err)
		}
		if err := t.ws.ValidateCase(c); err != nil {
			return harness.Result{}, err
		}
		bytes += c.Bytes
		made = append(made, caseView(c))
	}
	return harness.Result{
		Content:           mustJSON(map[string]any{"cases": made, "count": len(made)}),
		BytesMaterialized: bytes,
	}, nil
}

func (t *Tools) validateCases(ctx context.Context) (harness.Result, error) {
	cases := t.ws.Cases()
	if len(cases) == 0 {
		return harness.Result{}, fmt.Errorf("no cases materialized yet")
	}

	var problems []string
	for _, c := range cases {
		if err := t.ws.ValidateCase(c); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if t.ws.HasArtifact(ArtifactValidator) {
			if err := t.runValidator(ctx, c); err != nil {
				problems = append(problems, err.Error())
			}
		}
	}
	if len(problems) > 0 {
		return harness.Result{}, fmt.Errorf("case validation failed:\n%s", strings.Join(problems, "\n"))
	}
	return ok(map[string]any{"validated": len(cases)})
}

// runValidator applies the problem's own validator to a case. Polygon has this
// as a first-class artifact; AIOJ had nothing, so "n up to 1e5" was prose only.
func (t *Tools) runValidator(ctx context.Context, c Case) error {
	input, err := readCase(c)
	if err != nil {
		return err
	}
	v := t.ws.artifacts[ArtifactValidator]
	out, err := t.ws.runner.Run(ctx, v.compiled.Files, v.compiled.Args, input, Limits{TimeLimitMS: 10_000, MemoryLimitKB: 256 * 1024})
	if err != nil {
		return err
	}
	if !out.Accepted() {
		return fmt.Errorf("%s: validator rejected the input (%s) %s", c.Name, out.Status, capText(strings.TrimSpace(out.Stderr), 200))
	}
	return nil
}

func (t *Tools) runReference(ctx context.Context, args map[string]any) (harness.Result, error) {
	c, err := t.caseByName(strArg(args, "case_name"))
	if err != nil {
		return harness.Result{}, err
	}
	out, err := t.ws.RunReference(ctx, c)
	if err != nil {
		return harness.Result{}, err
	}
	if !out.Accepted() {
		return harness.Result{}, fmt.Errorf("reference solution failed on %s: %s after %d ms (limit %d ms)", c.Name, out.Status, out.TimeMS, t.ws.limits.TimeLimitMS)
	}
	return ok(map[string]any{
		"case":       c.Name,
		"verdict":    VerdictAC,
		"time_ms":    out.TimeMS,
		"memory_kb":  out.MemoryKB,
		"output_len": len(out.Stdout),
	})
}

// --- gates ----------------------------------------------------------------

func (t *Tools) checkSamples(ctx context.Context) (harness.Result, error) {
	checks := t.ws.CheckSamples(ctx, t.statement.Samples)
	if failed := failures(checks); len(failed) > 0 {
		return harness.Result{}, fmt.Errorf("sample verification failed:\n%s", strings.Join(failed, "\n"))
	}
	return ok(map[string]any{"checks": checks})
}

func (t *Tools) checkDeterminism(ctx context.Context, args map[string]any) (harness.Result, error) {
	c, err := t.caseByName(strArg(args, "case_name"))
	if err != nil {
		return harness.Result{}, err
	}
	check := t.ws.CheckDeterminism(ctx, c)
	if !check.Passed {
		return harness.Result{}, fmt.Errorf("%s", check.Message)
	}
	return ok(map[string]any{"check": check})
}

func (t *Tools) searchProblems(ctx context.Context, args map[string]any) (harness.Result, error) {
	if t.corpus == nil {
		return ok(map[string]any{"available": false, "note": "no corpus search is configured on this deployment"})
	}
	title := strArg(args, "title")
	if title == "" {
		return harness.Result{}, fmt.Errorf("title is required")
	}
	limit := intArg(args, "limit")
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	hits, err := t.corpus.Similar(ctx, title, strArg(args, "description"), limit)
	if err != nil {
		return harness.Result{}, err
	}
	var suspicious []SimilarProblem
	for _, h := range hits {
		if h.Similarity >= t.noveltyLimit {
			suspicious = append(suspicious, h)
		}
	}
	return ok(map[string]any{
		"available":        true,
		"hits":             hits,
		"likely_duplicate": suspicious,
		"threshold":        t.noveltyLimit,
	})
}

// finalize runs every gate. It is the only way a problem becomes ready: the
// model may choose the path, but not the bar.
func (t *Tools) finalize(ctx context.Context) (harness.Result, error) {
	report := Report{Ready: true}

	if _, found := t.ws.largestCase(); !found {
		report.Checks = append(report.Checks, Check{Name: "cases", Passed: false, Message: "no cases materialized"})
	}

	// Statement quality gates: the constraints field must be present, and the
	// hint must be empty (not a solution spoiler).
	if t.statement.Title != "" {
		hasConstraints := strings.TrimSpace(t.statement.Constraints) != ""
		report.Checks = append(report.Checks, Check{
			Name:    "constraints_declared",
			Passed:  hasConstraints,
			Message: map[bool]string{true: "constraints block is present", false: "the constraints field is empty — add a LaTeX bounds block listing every declared bound"}[hasConstraints],
		})
		if strings.TrimSpace(t.statement.Hint) != "" {
			report.Checks = append(report.Checks, Check{
				Name:    "hint_empty",
				Passed:  false,
				Message: "the hint field is not empty; it renders as a Notes section visible to contestants and must not contain the solution approach. Clear it.",
			})
		}
	}

	// Coverage first: it is cheap and its failure explains most others.
	report.Checks = append(report.Checks, t.ws.CheckCoverage()...)

	// Determinism on the largest case, where RNG use is most likely.
	if largest, ok := t.ws.largestCase(); ok {
		report.Checks = append(report.Checks, t.ws.CheckDeterminism(ctx, largest))
	}

	// Reference outputs for every case; without them the problem is not
	// judgeable at all.
	for _, c := range t.ws.Cases() {
		out, err := t.ws.RunReference(ctx, c)
		if err != nil {
			report.Checks = append(report.Checks, Check{Name: "reference", Passed: false, Message: err.Error()})
			break
		}
		if !out.Accepted() {
			report.Checks = append(report.Checks, Check{
				Name:    "reference",
				Passed:  false,
				Message: fmt.Sprintf("reference failed on %s (n=%d): %s", c.Name, c.N, out.Status),
			})
			break
		}
	}

	report.Checks = append(report.Checks, t.ws.CheckSamples(ctx, t.statement.Samples)...)
	report.Checks = append(report.Checks, t.ws.CheckLimits(ctx)...)
	report.Checks = append(report.Checks, t.crosscheckBrute(ctx)...)

	for _, c := range report.Checks {
		if !c.Passed {
			report.Ready = false
		}
	}
	report.Cases = t.ws.Cases()
	report.Blocked = report.Failed()

	if !report.Ready {
		return harness.Result{}, fmt.Errorf("not ready:\n%s", strings.Join(failureMessages(report.Checks), "\n"))
	}
	return ok(map[string]any{"report": report})
}

// crosscheckBrute compares the brute force against the reference on small
// cases. This is the only check that catches a wrong reference solution — the
// failure that actually makes a problem bad, and which no amount of
// determinism or limit checking will find.
func (t *Tools) crosscheckBrute(ctx context.Context) []Check {
	if !t.ws.HasArtifact(ArtifactBrute) {
		return nil
	}
	brute := t.ws.artifacts[ArtifactBrute]
	checked := 0
	for _, c := range t.ws.Cases() {
		if c.N > 1000 {
			continue // brute force is meant for small inputs only
		}
		input, err := readCase(c)
		if err != nil {
			return []Check{{Name: "crosscheck", Passed: false, Message: err.Error()}}
		}
		out, err := t.ws.runner.Run(ctx, brute.compiled.Files, brute.compiled.Args, input,
			Limits{TimeLimitMS: 10_000, MemoryLimitKB: 512 * 1024})
		if err != nil {
			return []Check{{Name: "crosscheck", Passed: false, Message: err.Error()}}
		}
		if !out.Accepted() {
			return []Check{{Name: "crosscheck", Passed: false, Message: fmt.Sprintf("brute force failed on %s: %s", c.Name, out.Status)}}
		}
		expected, err := os.ReadFile(strings.TrimSuffix(c.Path, ".in") + ".out")
		if err != nil {
			return []Check{{Name: "crosscheck", Passed: false, Message: fmt.Sprintf("%s has no expected output yet", c.Name)}}
		}
		if normalize(string(expected)) != normalize(out.Stdout) {
			return []Check{{
				Name:    "crosscheck",
				Passed:  false,
				Message: fmt.Sprintf("brute force disagrees with the reference on %s (n=%d) — one of them is wrong", c.Name, c.N),
			}}
		}
		checked++
	}
	if checked == 0 {
		return []Check{{Name: "crosscheck", Passed: false, Message: "no small cases available to cross-check"}}
	}
	return []Check{{Name: "crosscheck", Passed: true, Message: fmt.Sprintf("brute force agrees on %d small cases", checked)}}
}

// --- helpers --------------------------------------------------------------

func (t *Tools) caseByName(name string) (Case, error) {
	if name == "" {
		return Case{}, fmt.Errorf("case_name is required")
	}
	for _, c := range t.ws.Cases() {
		if c.Name == name {
			return c, nil
		}
	}
	return Case{}, fmt.Errorf("no such case %q", name)
}

// caseView is the model-facing projection of a case: metadata only, with the
// on-disk path stripped.
func caseView(c Case) map[string]any {
	return map[string]any{
		"name":       c.Name,
		"tier":       c.Tier,
		"n":          c.N,
		"shape":      c.Shape,
		"seed":       c.Seed,
		"bytes":      c.Bytes,
		"lines":      c.Lines,
		"sha256":     c.SHA256,
		"first_line": c.FirstLine,
		"last_line":  c.LastLine,
	}
}

func readCase(c Case) (string, error) {
	b, err := os.ReadFile(c.Path)
	if err != nil {
		return "", fmt.Errorf("read case %s: %w", c.Name, err)
	}
	return string(b), nil
}

func failures(checks []Check) []string {
	var out []string
	for _, c := range checks {
		if !c.Passed {
			out = append(out, c.Name+": "+c.Message)
		}
	}
	return out
}

func failureMessages(checks []Check) []string {
	var out []string
	for _, c := range checks {
		if !c.Passed {
			out = append(out, "- "+c.Name+": "+c.Message)
		}
	}
	return out
}

func ok(v map[string]any) (harness.Result, error) {
	v["ok"] = true
	return harness.Result{Content: mustJSON(v)}, nil
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"ok":false,"error":"failed to encode result"}`
	}
	return string(b)
}

func decodeArgs(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("arguments are not valid JSON: %w", err)
	}
	return m, nil
}

func strArg(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return strings.TrimSpace(s)
}

func intArg(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n
	default:
		return 0
	}
}

func strSlice(m map[string]any, k string) []string {
	raw, _ := m[k].([]any)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// --- schema builders ------------------------------------------------------

func object(props map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

func str(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func strings_(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
}

func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func arrayOf(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}
