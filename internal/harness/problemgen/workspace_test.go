package problemgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// fakeRunner emulates the sandbox without a live go-judge.
//
// It is a real enough generator to exercise the pipeline: it honours --n and
// --shape, and it can be told to misbehave in the ways real generated code
// does (ignore --n, reseed from the clock, emit a value out of range).
type fakeRunner struct {
	// compileFails makes every Compile return a failure with this log.
	compileFails string
	// behaviour selects the generator's defect.
	behaviour string
	// runs records every argv it saw, so tests can assert the contract.
	runs [][]string
}

func (f *fakeRunner) Compile(_ context.Context, language, source string) (CompileOutcome, error) {
	if f.compileFails != "" {
		return CompileOutcome{OK: false, Output: f.compileFails}, nil
	}
	if language == "" {
		return CompileOutcome{}, fmt.Errorf("unsupported language %q", language)
	}
	// The source doubles as the program: "FAIL" anywhere means a compile error.
	if strings.Contains(source, "COMPILE_ERROR") {
		return CompileOutcome{OK: false, Output: "error: expected ';'"}, nil
	}
	return CompileOutcome{
		OK:    true,
		Files: map[string]executor.CmdFile{"Main": {Content: source}},
		Args:  []string{"./Main"},
	}, nil
}

func (f *fakeRunner) Run(_ context.Context, files map[string]executor.CmdFile, args []string, stdin string, lim Limits) (RunOutcome, error) {
	f.runs = append(f.runs, args)
	source := files["Main"].Content

	// A solution echoes the answer the test encodes in its source.
	if strings.Contains(source, "SOLUTION") {
		if stdin == "" {
			return RunOutcome{Status: "Nonzero Exit Status", Stderr: "no input"}, nil
		}
		first := strings.Fields(stdin)[0]
		return RunOutcome{Status: "Accepted", Stdout: first + "\n", TimeMS: 5}, nil
	}
	if strings.Contains(source, "BRUTE") {
		first := strings.Fields(stdin)[0]
		return RunOutcome{Status: "Accepted", Stdout: first + "\n", TimeMS: 5}, nil
	}
	if strings.Contains(source, "VALIDATOR") {
		return RunOutcome{Status: "Accepted"}, nil
	}

	// Otherwise it is the generator.
	n := argValue(args, "--n")
	shape := argValue(args, "--shape")
	seed := argValue(args, "--seed")

	switch f.behaviour {
	case "ignore-n":
		n = "7" // ignores what it was asked for
	}
	if n == "" {
		return RunOutcome{Status: "Nonzero Exit Status", Stderr: "missing --n"}, nil
	}

	// Emit n integers; the shape controls the values so a shape-ignoring
	// generator can be detected by identical output.
	base := int64(1)
	switch shape {
	case "reverse":
		base = -1
	case "zero":
		base = 0
	}
	if f.behaviour == "out-of-range" {
		base = 1_000_000_000
	}
	if f.behaviour == "ignore-shape" {
		base = 1 // identical bytes regardless of --shape
	}
	if f.behaviour == "nondeterministic" {
		// Emulate a generator that reseeds from the clock: the output depends
		// on something other than the requested seed, so two runs differ.
		base = int64(len(f.runs) + 100)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", n)
	count := atoi(n)
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d", base)
	}
	b.WriteByte('\n')
	_ = seed
	return RunOutcome{Status: "Accepted", Stdout: b.String(), TimeMS: 3}, nil
}

func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func atoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func newTestWorkspace(t *testing.T, runner Runner) *Workspace {
	t.Helper()
	ws, err := NewWorkspace(WorkspaceOptions{
		Root:    t.TempDir(),
		JobID:   "job-1",
		Runner:  runner,
		MaxByte: 8 << 20,
	})
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	return ws
}

func mustConstraints(t *testing.T, ws *Workspace) {
	t.Helper()
	if err := ws.SetConstraints(Constraints{NMin: 1, NMax: 100000, ValueMin: 0, ValueMax: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := ws.SetLimits(Limits{TimeLimitMS: 2000, MemoryLimitKB: 262144}); err != nil {
		t.Fatal(err)
	}
}

func writeGen(t *testing.T, ws *Workspace) {
	t.Helper()
	if _, err := ws.WriteArtifact(context.Background(), ArtifactGenerator, "cpp-gpp-64", "GENERATOR"); err != nil {
		t.Fatalf("write generator: %v", err)
	}
}

func TestMatrixDerivesTiersFromConstraints(t *testing.T) {
	// The model never chooses the matrix: left alone it produces similar random
	// cases and no max case, which is the one that proves the constraint bites.
	got := Matrix(Constraints{NMin: 1, NMax: 100000}, []string{"random", "sorted"}, 1)

	var tiers []Tier
	for _, e := range got {
		tiers = append(tiers, e.Tier)
	}
	for _, want := range []Tier{TierMin, TierSmall, TierMedium, TierMax, TierPathological} {
		var found bool
		for _, have := range tiers {
			if have == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("tier %q missing from the matrix: %v", want, tiers)
		}
	}

	// 5 tiers x 2 shapes x 1 seed
	if len(got) != 10 {
		t.Fatalf("matrix has %d entries, want 10", len(got))
	}
	// Every entry must carry the declared n_max for the max tier.
	for _, e := range got {
		if e.Tier == TierMax && e.N != 100000 {
			t.Fatalf("max tier n = %d, want 100000", e.N)
		}
	}
}

func TestMatrixClampsSmallTiersToTheDeclaredRange(t *testing.T) {
	got := Matrix(Constraints{NMin: 1, NMax: 5}, []string{"random"}, 1)
	for _, e := range got {
		if e.N > 5 {
			t.Fatalf("entry %+v exceeds n_max", e)
		}
	}
}

func TestMaterializeProducesLargeCasesFromCode(t *testing.T) {
	runner := &fakeRunner{}
	ws := newTestWorkspace(t, runner)
	mustConstraints(t, ws)
	writeGen(t, ws)

	c, err := ws.Materialize(context.Background(), MatrixEntry{Tier: TierMax, N: 100000, Shape: "random", Seed: 1})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	// The point of the whole design: a 1e5 case exists on disk even though no
	// model response could ever contain it (max_tokens defaults to 8192).
	if c.N != 100000 {
		t.Fatalf("n = %d", c.N)
	}
	if c.Bytes < 100000 {
		t.Fatalf("case is only %d bytes; expected a full 1e5-line input", c.Bytes)
	}
	if c.Lines < 2 {
		t.Fatalf("lines = %d", c.Lines)
	}
	if c.SHA256 == "" {
		t.Fatal("case has no hash")
	}
	info, err := os.Stat(c.Path)
	if err != nil {
		t.Fatalf("case file missing: %v", err)
	}
	if info.Size() != c.Bytes {
		t.Fatalf("recorded %d bytes but the file is %d", c.Bytes, info.Size())
	}

	// The contract: flags, not positional arguments.
	if got := argValue(runner.runs[0], "--n"); got != "100000" {
		t.Fatalf("--n = %q, want 100000", got)
	}
	if got := argValue(runner.runs[0], "--shape"); got != "random" {
		t.Fatalf("--shape = %q", got)
	}
}

func TestValidateCaseRejectsAGeneratorThatIgnoresN(t *testing.T) {
	// A well-formed case describing a different problem than the statement is
	// the failure mode that a plain "did it run" check cannot see.
	runner := &fakeRunner{behaviour: "ignore-n"}
	ws := newTestWorkspace(t, runner)
	mustConstraints(t, ws)
	writeGen(t, ws)

	c, err := ws.Materialize(context.Background(), MatrixEntry{Tier: TierMedium, N: 1000, Shape: "random", Seed: 1})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	err = ws.ValidateCase(c)
	if err == nil {
		t.Fatal("expected a violation")
	}
	if !strings.Contains(err.Error(), "was asked for n=1000") {
		t.Fatalf("unhelpful error: %v", err)
	}
}

func TestValidateCaseRejectsOutOfRangeValues(t *testing.T) {
	runner := &fakeRunner{behaviour: "out-of-range"}
	ws := newTestWorkspace(t, runner)
	mustConstraints(t, ws) // value range [0, 1000]
	writeGen(t, ws)

	c, err := ws.Materialize(context.Background(), MatrixEntry{Tier: TierSmall, N: 10, Shape: "random", Seed: 1})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if err := ws.ValidateCase(c); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("err = %v, want a range violation", err)
	}
}

func TestCheckDeterminismCatchesAClockSeededGenerator(t *testing.T) {
	runner := &fakeRunner{behaviour: "nondeterministic"}
	ws := newTestWorkspace(t, runner)
	mustConstraints(t, ws)
	writeGen(t, ws)

	c, err := ws.Materialize(context.Background(), MatrixEntry{Tier: TierSmall, N: 10, Shape: "random", Seed: 1})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	check := ws.CheckDeterminism(context.Background(), c)
	if check.Passed {
		t.Fatal("a generator that reseeds itself must fail the determinism check")
	}
	if !strings.Contains(check.Message, "seed its RNG from --seed") {
		t.Fatalf("message should tell the model how to fix it: %q", check.Message)
	}
}

func TestCheckDeterminismPassesForAStableGenerator(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	mustConstraints(t, ws)
	writeGen(t, ws)

	c, _ := ws.Materialize(context.Background(), MatrixEntry{Tier: TierSmall, N: 10, Shape: "random", Seed: 4})
	if check := ws.CheckDeterminism(context.Background(), c); !check.Passed {
		t.Fatalf("unexpected failure: %s", check.Message)
	}
}

func TestCheckSamplesCatchesAWrongStatementExample(t *testing.T) {
	// Nothing verified this before: a statement could ship with wrong examples
	// while the judge stayed internally consistent.
	ws := newTestWorkspace(t, &fakeRunner{})
	mustConstraints(t, ws)
	if _, err := ws.WriteArtifact(context.Background(), ArtifactSolution, "cpp-gpp-64", "SOLUTION"); err != nil {
		t.Fatal(err)
	}

	// The fake solution echoes the first token, so "3" -> "3".
	checks := ws.CheckSamples(context.Background(), []Sample{{Input: "3 1 2 3", Output: "3"}})
	if len(checks) != 1 || !checks[0].Passed {
		t.Fatalf("a correct sample must pass: %+v", checks)
	}

	checks = ws.CheckSamples(context.Background(), []Sample{{Input: "3 1 2 3", Output: "6"}})
	if len(checks) != 1 || checks[0].Passed {
		t.Fatalf("a wrong sample must fail: %+v", checks)
	}
	if !strings.Contains(checks[0].Message, "expected") {
		t.Fatalf("message should show both values: %q", checks[0].Message)
	}
}

func TestCheckSamplesRequiresAtLeastOneSample(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	mustConstraints(t, ws)
	checks := ws.CheckSamples(context.Background(), nil)
	if len(checks) != 1 || checks[0].Passed {
		t.Fatalf("checks = %+v, want a failure", checks)
	}
}

func TestCheckCoverageFlagsAGeneratorThatIgnoresShape(t *testing.T) {
	// Same n, two shapes, identical bytes: the shape argument is being ignored,
	// so the problem is only ever tested on one kind of input.
	ws := newTestWorkspace(t, &fakeRunner{behaviour: "ignore-shape"})
	mustConstraints(t, ws)
	writeGen(t, ws)

	for i, shape := range []string{"random", "reverse"} {
		if _, err := ws.Materialize(context.Background(), MatrixEntry{Tier: TierMax, N: 100, Shape: shape, Seed: int64(i + 1)}); err != nil {
			t.Fatalf("materialize: %v", err)
		}
	}
	// A min case so only diversity can fail.
	if _, err := ws.Materialize(context.Background(), MatrixEntry{Tier: TierMin, N: 1, Shape: "random", Seed: 9}); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	var sawDiversity bool
	for _, c := range ws.CheckCoverage() {
		if c.Name == "case_diversity" && !c.Passed {
			sawDiversity = true
			if !strings.Contains(c.Message, "ignores its arguments") {
				t.Fatalf("message should name the defect: %q", c.Message)
			}
		}
	}
	if !sawDiversity {
		t.Fatalf("a shape-ignoring generator must be flagged: %+v", ws.CheckCoverage())
	}
}

func TestCheckCoverageAllowsIdenticalBytesWithinOneCell(t *testing.T) {
	// A small n_max makes the medium and max tiers the same request, so the
	// same seed legitimately produces the same bytes. That is not a defect.
	ws := newTestWorkspace(t, &fakeRunner{})
	if err := ws.SetConstraints(Constraints{NMin: 1, NMax: 10, ValueMin: 0, ValueMax: 1000}); err != nil {
		t.Fatal(err)
	}
	writeGen(t, ws)

	for i, tier := range []Tier{TierMin, TierMedium, TierMax} {
		if _, err := ws.Materialize(context.Background(), MatrixEntry{Tier: tier, N: 10, Shape: "random", Seed: int64(i + 1)}); err != nil {
			t.Fatalf("materialize: %v", err)
		}
	}
	for _, c := range ws.CheckCoverage() {
		if c.Name == "case_diversity" && !c.Passed {
			t.Fatalf("same-cell duplicates must not be flagged: %s", c.Message)
		}
	}
}

func TestRunReferenceWritesExpectedOutput(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	mustConstraints(t, ws)
	writeGen(t, ws)
	if _, err := ws.WriteArtifact(context.Background(), ArtifactSolution, "cpp-gpp-64", "SOLUTION"); err != nil {
		t.Fatal(err)
	}

	c, _ := ws.Materialize(context.Background(), MatrixEntry{Tier: TierSmall, N: 10, Shape: "random", Seed: 1})
	out, err := ws.RunReference(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted() {
		t.Fatalf("reference failed: %+v", out)
	}

	outPath := strings.TrimSuffix(c.Path, ".in") + ".out"
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected output not written: %v", err)
	}
	if strings.TrimSpace(string(data)) != "10" {
		t.Fatalf("expected output = %q", string(data))
	}
}

func TestWriteArtifactReportsCompileErrors(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	out, err := ws.WriteArtifact(context.Background(), ArtifactGenerator, "cpp-gpp-64", "COMPILE_ERROR")
	if err != nil {
		t.Fatal(err)
	}
	if out.OK {
		t.Fatal("expected a compile failure")
	}
	// The source must still be kept: a failed compile is worth keeping for the
	// audit trail and a later retry.
	if _, err := os.Stat(filepath.Join(ws.Root(), "generator.txt")); err != nil {
		t.Fatalf("source not persisted: %v", err)
	}
	if ws.HasArtifact(ArtifactGenerator) {
		t.Fatal("a failed compile must not count as a usable artifact")
	}
}

func TestMaterializeRequiresConstraints(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	writeGen(t, ws)
	if _, err := ws.Materialize(context.Background(), MatrixEntry{Tier: TierSmall, N: 10}); err == nil {
		t.Fatal("expected an error without constraints")
	}
}

func TestWorkspaceRejectsPathTraversalInJobID(t *testing.T) {
	root := t.TempDir()
	ws, err := NewWorkspace(WorkspaceOptions{Root: root, JobID: "../../etc", Runner: &fakeRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Cleanup()
	// filepath.Base strips the traversal, so the directory stays under root.
	if !strings.HasPrefix(ws.Root(), root) {
		t.Fatalf("workspace escaped its root: %s", ws.Root())
	}
}

func TestCasePathOnlyReturnsKnownCases(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	mustConstraints(t, ws)
	writeGen(t, ws)
	c, _ := ws.Materialize(context.Background(), MatrixEntry{Tier: TierSmall, N: 10, Shape: "random", Seed: 1})

	if _, ok := ws.CasePath(c.Name); !ok {
		t.Fatal("a materialized case must be findable")
	}
	if _, ok := ws.CasePath("../../../etc/passwd"); ok {
		t.Fatal("an unknown name must not resolve to a path")
	}
}
