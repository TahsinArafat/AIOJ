package problemgen

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Workspace is the per-job sandbox scratch area and the bookkeeping for one
// problem under construction.
//
// It owns every filesystem path. The model never supplies one: case names are
// generated here, so a crafted tool argument cannot escape the staging root.
type Workspace struct {
	root        string
	runner      Runner
	constraints Constraints
	limits      Limits
	maxBytes    int64

	artifacts map[Artifact]artifactState
	cases     []Case
	usage     Usage
}

type artifactState struct {
	source   string
	language string
	compiled CompileOutcome
}

// WorkspaceOptions configures a workspace.
type WorkspaceOptions struct {
	// Root is the parent directory; a per-job subdirectory is created inside.
	Root string
	// JobID names the subdirectory, so concurrent jobs cannot collide.
	JobID   string
	Runner  Runner
	MaxByte int64
}

// NewWorkspace creates (or reopens) the staging directory for a job.
func NewWorkspace(opts WorkspaceOptions) (*Workspace, error) {
	if opts.Runner == nil {
		return nil, errors.New("problemgen: a runner is required")
	}
	if opts.Root == "" {
		return nil, errors.New("problemgen: a staging root is required")
	}
	if opts.JobID == "" {
		return nil, errors.New("problemgen: a job id is required")
	}
	// filepath.Base strips any traversal an upstream caller might have supplied.
	root := filepath.Join(opts.Root, filepath.Base(opts.JobID))
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("problemgen: create staging dir: %w", err)
	}
	maxBytes := opts.MaxByte
	if maxBytes <= 0 {
		maxBytes = 64 << 20
	}
	return &Workspace{
		root:      root,
		runner:    opts.Runner,
		maxBytes:  maxBytes,
		artifacts: make(map[Artifact]artifactState),
	}, nil
}

// Root is the staging directory.
func (w *Workspace) Root() string { return w.root }

// SetConstraints records the problem's bounds.
func (w *Workspace) SetConstraints(c Constraints) error {
	if err := c.Validate(); err != nil {
		return err
	}
	w.constraints = c
	return nil
}

// SetLimits records the judge limits.
func (w *Workspace) SetLimits(l Limits) error {
	if err := l.Validate(); err != nil {
		return err
	}
	w.limits = l
	return nil
}

// Constraints and Limits expose the current values.
func (w *Workspace) Constraints() Constraints { return w.constraints }
func (w *Workspace) Limits() Limits           { return w.limits }
func (w *Workspace) Usage() Usage             { return w.usage }
func (w *Workspace) Cases() []Case            { return w.cases }

// WriteArtifact stores a source file and compiles it.
//
// Compiling on write (rather than lazily at finalize) is deliberate: the model
// gets a compile error on the turn it wrote the code, which is the cheapest
// moment to fix it.
func (w *Workspace) WriteArtifact(ctx context.Context, kind Artifact, language, source string) (CompileOutcome, error) {
	if !validArtifact(kind) {
		return CompileOutcome{}, fmt.Errorf("unknown artifact %q", kind)
	}
	if strings.TrimSpace(source) == "" {
		return CompileOutcome{}, fmt.Errorf("%s source is empty", kind)
	}
	if int64(len(source)) > 1<<20 {
		return CompileOutcome{}, fmt.Errorf("%s source exceeds 1 MiB", kind)
	}

	// Persist the source first: even a failed compile is worth keeping for the
	// audit trail and for a later retry.
	path := filepath.Join(w.root, string(kind)+".txt")
	if err := os.WriteFile(path, []byte(source), 0o640); err != nil {
		return CompileOutcome{}, fmt.Errorf("write %s: %w", kind, err)
	}

	out, err := w.runner.Compile(ctx, language, source)
	if err != nil {
		return CompileOutcome{}, err
	}
	w.usage.Compilations++
	w.artifacts[kind] = artifactState{source: source, language: language, compiled: out}
	return out, nil
}

// ArtifactSource returns the stored source for a kind.
func (w *Workspace) ArtifactSource(kind Artifact) (string, bool) {
	a, ok := w.artifacts[kind]
	return a.source, ok
}

// ArtifactLanguage returns the language a kind was written in.
func (w *Workspace) ArtifactLanguage(kind Artifact) string {
	return w.artifacts[kind].language
}

// HasArtifact reports whether a kind was written and compiled successfully.
func (w *Workspace) HasArtifact(kind Artifact) bool {
	a, ok := w.artifacts[kind]
	return ok && a.compiled.OK
}

// MatrixEntry is one case the pipeline will materialize.
type MatrixEntry struct {
	Tier  Tier
	N     int
	Shape string
	Seed  int64
}

// Matrix derives the case matrix from the constraints and the generator's
// declared shapes.
//
// The model does not choose the matrix. Left to its own devices it produces a
// handful of similar random cases and no maximum case — the one case that
// proves the constraint actually bites. Tiers are computed here so every
// problem gets the same coverage.
func Matrix(c Constraints, shapes []string, seedsPerCell int) []MatrixEntry {
	if len(shapes) == 0 {
		shapes = []string{"random"}
	}
	if seedsPerCell < 1 {
		seedsPerCell = 1
	}

	sizes := []struct {
		tier Tier
		n    int
	}{
		{TierMin, c.NMin},
		{TierSmall, clampN(10, c)},
		{TierMedium, clampN(1000, c)},
		{TierMax, c.NMax},
	}
	// A pathological max case is what breaks heuristics that pass on random
	// data, so it is part of the default matrix rather than an optional extra.
	if c.NMax > 1 {
		sizes = append(sizes, struct {
			tier Tier
			n    int
		}{TierPathological, c.NMax})
	}

	var out []MatrixEntry
	seed := int64(1)
	for _, s := range sizes {
		for _, shape := range shapes {
			for i := 0; i < seedsPerCell; i++ {
				out = append(out, MatrixEntry{Tier: s.tier, N: s.n, Shape: shape, Seed: seed})
				seed++
			}
		}
	}
	return out
}

func clampN(n int, c Constraints) int {
	if n < c.NMin {
		return c.NMin
	}
	if n > c.NMax {
		return c.NMax
	}
	return n
}

// Materialize runs the generator for one matrix entry and records the case.
//
// This is the crux of the whole design: the bytes are produced by executing
// generated code inside the sandbox. A 1e5-line input is ~1 MB and could never
// be typed by the model into a response (max_tokens defaults to 8192).
func (w *Workspace) Materialize(ctx context.Context, entry MatrixEntry) (Case, error) {
	if !w.HasArtifact(ArtifactGenerator) {
		return Case{}, errors.New("generator is not compiled yet")
	}
	if w.constraints.NMax <= 0 {
		return Case{}, errors.New("constraints are not set yet")
	}
	if w.usage.BytesWritten >= w.maxBytes {
		return Case{}, fmt.Errorf("materialized data budget exhausted (%d bytes)", w.maxBytes)
	}

	gen := w.artifacts[ArtifactGenerator]
	args := append(append([]string{}, gen.compiled.Args...),
		"--seed", strconv.FormatInt(entry.Seed, 10),
		"--n", strconv.Itoa(entry.N),
		"--shape", entry.Shape,
	)

	// The generator gets a generous slice of CPU but its own, smaller memory
	// budget: it must not be able to take the judge host down while producing
	// data for a problem that is itself cheap to solve.
	genLimits := Limits{TimeLimitMS: 10_000, MemoryLimitKB: 256 * 1024}
	out, err := w.runner.Run(ctx, gen.compiled.Files, args, "", genLimits)
	if err != nil {
		return Case{}, err
	}
	if !out.Accepted() {
		return Case{}, fmt.Errorf("generator failed for %s/%s: status %s %s",
			entry.Tier, entry.Shape, out.Status, capText(strings.TrimSpace(out.Stderr), 400))
	}
	if strings.TrimSpace(out.Stdout) == "" {
		return Case{}, fmt.Errorf("generator produced an empty case for %s/%s", entry.Tier, entry.Shape)
	}

	name := fmt.Sprintf("%s-%s-%02d.in", entry.Tier, sanitize(entry.Shape), entry.Seed)
	path := filepath.Join(w.root, name)
	if err := os.WriteFile(path, []byte(out.Stdout), 0o640); err != nil {
		return Case{}, fmt.Errorf("write case: %w", err)
	}

	c := Case{
		Name:   name,
		Seed:   entry.Seed,
		N:      entry.N,
		Shape:  entry.Shape,
		Tier:   entry.Tier,
		Bytes:  int64(len(out.Stdout)),
		Path:   path,
		SHA256: hashBytes([]byte(out.Stdout)),
	}
	c.Lines, c.FirstLine, c.LastLine = describe(out.Stdout)

	w.usage.BytesWritten += c.Bytes
	w.usage.Materialized++
	w.cases = append(w.cases, c)
	return c, nil
}

// ValidateCase checks a materialized case against the declared constraints.
//
// This is the check that catches a generator which ignores --n: the case is
// well-formed but describes a different problem than the statement claims.
func (w *Workspace) ValidateCase(c Case) error {
	if c.N < w.constraints.NMin || c.N > w.constraints.NMax {
		return fmt.Errorf("%s: n=%d outside [%d,%d]", c.Name, c.N, w.constraints.NMin, w.constraints.NMax)
	}

	data, err := os.ReadFile(c.Path)
	if err != nil {
		return fmt.Errorf("%s: %w", c.Name, err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return fmt.Errorf("%s: empty", c.Name)
	}

	declared, err := strconv.Atoi(fields[0])
	if err != nil {
		return fmt.Errorf("%s: first token %q is not an integer", c.Name, capText(fields[0], 32))
	}
	if declared != c.N {
		return fmt.Errorf("%s: generator declared n=%d but was asked for n=%d", c.Name, declared, c.N)
	}
	for _, f := range fields[1:] {
		v, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: non-integer token %q", c.Name, capText(f, 32))
		}
		if v < w.constraints.ValueMin || v > w.constraints.ValueMax {
			return fmt.Errorf("%s: value %d outside [%d,%d]", c.Name, v, w.constraints.ValueMin, w.constraints.ValueMax)
		}
	}
	return nil
}

// RunReference executes the reference solution on a case and writes its output
// alongside the input, which is what makes the case judgeable.
func (w *Workspace) RunReference(ctx context.Context, c Case) (RunOutcome, error) {
	if !w.HasArtifact(ArtifactSolution) {
		return RunOutcome{}, errors.New("reference solution is not compiled yet")
	}
	if err := w.limits.Validate(); err != nil {
		return RunOutcome{}, err
	}

	input, err := os.ReadFile(c.Path)
	if err != nil {
		return RunOutcome{}, err
	}
	sol := w.artifacts[ArtifactSolution]
	out, err := w.runner.Run(ctx, sol.compiled.Files, sol.compiled.Args, string(input), w.limits)
	if err != nil {
		return RunOutcome{}, err
	}
	w.usage.ReferenceRuns++

	if !out.Accepted() {
		return out, nil // the caller decides whether that is a failure
	}
	outPath := strings.TrimSuffix(c.Path, ".in") + ".out"
	if err := os.WriteFile(outPath, []byte(out.Stdout), 0o640); err != nil {
		return out, fmt.Errorf("write expected output: %w", err)
	}
	w.usage.BytesWritten += int64(len(out.Stdout))
	return out, nil
}

// CheckSamples verifies the statement's samples against the reference
// solution. Nothing did this before: a statement could ship with wrong examples
// while the judge stayed internally consistent.
func (w *Workspace) CheckSamples(ctx context.Context, samples []Sample) []Check {
	check := Check{Name: "samples", Passed: true}
	if len(samples) == 0 {
		return []Check{{Name: "samples", Passed: false, Message: "no samples declared"}}
	}
	if !w.HasArtifact(ArtifactSolution) {
		return []Check{{Name: "samples", Passed: false, Message: "reference solution is not compiled"}}
	}

	sol := w.artifacts[ArtifactSolution]
	for i, s := range samples {
		out, err := w.runner.Run(ctx, sol.compiled.Files, sol.compiled.Args, s.Input, w.limits)
		if err != nil {
			return []Check{{Name: "samples", Passed: false, Message: fmt.Sprintf("sample %d: %v", i+1, err)}}
		}
		if !out.Accepted() {
			return []Check{{Name: "samples", Passed: false, Message: fmt.Sprintf("sample %d: reference solution failed (%s)", i+1, out.Status)}}
		}
		if normalize(s.Output) != normalize(out.Stdout) {
			return []Check{{
				Name:    "samples",
				Passed:  false,
				Message: fmt.Sprintf("sample %d: expected %q, got %q", i+1, trim(s.Output, 80), trim(out.Stdout, 80)),
			}}
		}
	}
	return []Check{check}
}

// CheckLimits runs the reference on the largest case under the declared limits.
//
// This is the only check that validates constraints, solution and limits
// together. If the intended solution cannot pass its own maximum case, the
// problem is unsolvable as specified.
func (w *Workspace) CheckLimits(ctx context.Context) []Check {
	if !w.HasArtifact(ArtifactSolution) {
		return []Check{{Name: "limits", Passed: false, Message: "reference solution is not compiled"}}
	}
	largest, ok := w.largestCase()
	if !ok {
		return []Check{{Name: "limits", Passed: false, Message: "no max case materialized"}}
	}

	input, err := os.ReadFile(largest.Path)
	if err != nil {
		return []Check{{Name: "limits", Passed: false, Message: err.Error()}}
	}
	sol := w.artifacts[ArtifactSolution]
	out, err := w.runner.Run(ctx, sol.compiled.Files, sol.compiled.Args, string(input), w.limits)
	if err != nil {
		return []Check{{Name: "limits", Passed: false, Message: err.Error()}}
	}
	if !out.Accepted() {
		return []Check{{
			Name:    "limits",
			Passed:  false,
			Message: fmt.Sprintf("reference solution does not pass %s (n=%d) within %d ms / %d KB: %s", largest.Name, largest.N, w.limits.TimeLimitMS, w.limits.MemoryLimitKB, out.Status),
		}}
	}

	checks := []Check{{Name: "limits", Passed: true, Message: fmt.Sprintf("reference passed %s in %d ms", largest.Name, out.TimeMS)}}
	// A solution that eats most of the budget will TLE on a slower judge.
	if w.limits.TimeLimitMS > 0 && out.TimeMS*100/w.limits.TimeLimitMS >= 60 {
		checks = append(checks, Check{
			Name:    "limits_headroom",
			Passed:  true,
			Warning: true,
			Message: fmt.Sprintf("reference uses %d ms of %d ms (%d%%) — little headroom for judge noise", out.TimeMS, w.limits.TimeLimitMS, out.TimeMS*100/w.limits.TimeLimitMS),
		})
	}
	return checks
}

// CheckDeterminism verifies the generator repeats itself for a given seed.
//
// A generator seeded from the clock or from an unseeded rand() is the single
// most common defect in generated problem code, and it makes every downstream
// guarantee meaningless: the expected output no longer matches the input the
// judge will later produce.
func (w *Workspace) CheckDeterminism(ctx context.Context, c Case) Check {
	if !w.HasArtifact(ArtifactGenerator) {
		return Check{Name: "determinism", Passed: false, Message: "generator is not compiled"}
	}
	gen := w.artifacts[ArtifactGenerator]
	args := append(append([]string{}, gen.compiled.Args...),
		"--seed", strconv.FormatInt(c.Seed, 10),
		"--n", strconv.Itoa(c.N),
		"--shape", c.Shape,
	)
	genLimits := Limits{TimeLimitMS: 10_000, MemoryLimitKB: 256 * 1024}

	first, err := w.runner.Run(ctx, gen.compiled.Files, args, "", genLimits)
	if err != nil {
		return Check{Name: "determinism", Passed: false, Message: err.Error()}
	}
	second, err := w.runner.Run(ctx, gen.compiled.Files, args, "", genLimits)
	if err != nil {
		return Check{Name: "determinism", Passed: false, Message: err.Error()}
	}
	if first.Stdout != second.Stdout {
		return Check{
			Name:    "determinism",
			Passed:  false,
			Message: fmt.Sprintf("generator produced different output for seed %d (n=%d, shape %s) — it must seed its RNG from --seed", c.Seed, c.N, c.Shape),
		}
	}
	return Check{Name: "determinism", Passed: true, Message: fmt.Sprintf("stable across runs for seed %d", c.Seed)}
}

// CheckCoverage verifies the case set has the shape a problem needs.
func (w *Workspace) CheckCoverage() []Check {
	if len(w.cases) == 0 {
		return []Check{{Name: "coverage", Passed: false, Message: "no cases materialized"}}
	}

	var checks []Check
	seen := make(map[Tier]bool, len(TierOrder))
	for _, c := range w.cases {
		seen[c.Tier] = true
	}

	// The max case is the one that proves the constraint bites; without it the
	// problem is only tested on inputs a wrong solution also passes.
	var missing []string
	if !seen[TierMax] && !seen[TierPathological] {
		missing = append(missing, "max")
	}
	if !seen[TierMin] {
		missing = append(missing, "min")
	}
	if len(missing) > 0 {
		checks = append(checks, Check{Name: "coverage", Passed: false, Message: "missing case tiers: " + strings.Join(missing, ", ")})
	} else {
		checks = append(checks, Check{Name: "coverage", Passed: true, Message: fmt.Sprintf("%d cases across %d tiers", len(w.cases), len(seen))})
	}

	// Byte-identical cases across *different* (n, shape) cells hide a generator
	// that ignores one of its arguments. Within a cell it is expected: a small
	// n_max makes the medium and max tiers the same request, and the same seed
	// must then produce the same bytes.
	hashToCell := make(map[string]string, len(w.cases))
	var dupes []string
	for _, c := range w.cases {
		cell := fmt.Sprintf("n=%d shape=%s", c.N, c.Shape)
		if prevCell, ok := hashToCell[c.SHA256]; ok && prevCell != cell {
			dupes = append(dupes, fmt.Sprintf("%s and %s produced identical output", prevCell, cell))
			continue
		}
		hashToCell[c.SHA256] = cell
	}
	if len(dupes) > 0 {
		checks = append(checks, Check{Name: "case_diversity", Passed: false, Message: "the generator ignores its arguments: " + strings.Join(dupes, "; ")})
	}
	return checks
}

func (w *Workspace) largestCase() (Case, bool) {
	var best Case
	found := false
	for _, c := range w.cases {
		if !found || c.N > best.N || (c.N == best.N && c.Bytes > best.Bytes) {
			best, found = c, true
		}
	}
	return best, found
}

// CasePath returns the on-disk path for a case by name, rejecting anything that
// is not a case this workspace created.
func (w *Workspace) CasePath(name string) (string, bool) {
	for _, c := range w.cases {
		if c.Name == name {
			return c.Path, true
		}
	}
	return "", false
}

// Cleanup removes the staging directory.
func (w *Workspace) Cleanup() error {
	if w.root == "" {
		return nil
	}
	return os.RemoveAll(w.root)
}

func validArtifact(a Artifact) bool {
	for _, x := range AllArtifacts {
		if x == a {
			return true
		}
	}
	return false
}

// describe returns line count and bounded first/last lines for a case body.
// The model sees this instead of the bytes: a 1 MB input has no place in a
// context window, and the metadata is enough to reason about the case.
func describe(s string) (lines int, first, last string) {
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		line := sc.Text()
		if lines == 0 {
			first = trim(line, 120)
		}
		last = trim(line, 120)
		lines++
	}
	return lines, first, last
}

func trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// normalize collapses whitespace so trailing newlines do not fail a comparison.
func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		}
	}
	if b.Len() == 0 {
		return "case"
	}
	return b.String()
}

// elapsed is a small helper for reporting.
func elapsed(start time.Time) time.Duration { return time.Since(start) }
