package problemgen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// MaxN bounds n_max so a single case cannot exhaust the judge host's memory:
// the judge loads a whole input into memory and ships it as sandbox CopyIn, so
// an unbounded n is a host-memory risk, not only a disk one. Overridden at
// startup from AI_MAX_CASE_N.
var MaxN = 1_000_000

// Constraints is the machine-readable statement of a problem's bounds.
//
// It is the single source of truth for three consumers, which is why it is a
// declared artifact rather than prose in the statement:
//
//  1. the case matrix sizes its max case from NMax;
//  2. the statement's constraint block is rendered from these numbers;
//  3. every generated case is validated against them.
//
// Without it, "n up to 1e5" lives only in text the model wrote, and nothing can
// check that the generator honoured it.
type Constraints struct {
	NMin     int   `json:"n_min"`
	NMax     int   `json:"n_max"`
	SumNMax  int   `json:"sum_n_max,omitempty"`
	ValueMin int64 `json:"value_min"`
	ValueMax int64 `json:"value_max"`
}

// Validate checks internal consistency.
func (c Constraints) Validate() error {
	if c.NMin < 1 {
		return fmt.Errorf("n_min must be at least 1")
	}
	if c.NMax < c.NMin {
		return fmt.Errorf("n_max (%d) must be >= n_min (%d)", c.NMax, c.NMin)
	}
	if MaxN > 0 && c.NMax > MaxN {
		return fmt.Errorf("n_max (%d) exceeds the configured limit of %d", c.NMax, MaxN)
	}
	if c.ValueMin > c.ValueMax {
		return fmt.Errorf("value_min must not exceed value_max")
	}
	return nil
}

// Limits are the judge limits the reference solution must satisfy.
type Limits struct {
	TimeLimitMS   int `json:"time_limit_ms"`
	MemoryLimitKB int `json:"memory_limit_kb"`
}

// Validate rejects the values that would make a problem unjudgeable. A zero
// time limit is not "unlimited" in this judge — it becomes the CPU limit — so
// it is rejected rather than defaulted silently.
func (l Limits) Validate() error {
	if l.TimeLimitMS <= 0 {
		return fmt.Errorf("time_limit_ms must be positive")
	}
	if l.TimeLimitMS > 60_000 {
		return fmt.Errorf("time_limit_ms must be at most 60000")
	}
	if l.MemoryLimitKB <= 0 {
		return fmt.Errorf("memory_limit_kb must be positive")
	}
	if l.MemoryLimitKB > 4<<20 {
		return fmt.Errorf("memory_limit_kb must be at most 4194304 (4 GiB)")
	}
	return nil
}

// Tier names the intent of a generated case. The matrix is derived from
// constraints, never chosen by the model: letting the model pick is how you end
// up with twelve identical random cases and no max case.
type Tier string

const (
	TierMin          Tier = "min"
	TierSmall        Tier = "small"
	TierMedium       Tier = "medium"
	TierMax          Tier = "max"
	TierPathological Tier = "pathological"
)

// TierOrder is the order tiers are reported in.
var TierOrder = []Tier{TierMin, TierSmall, TierMedium, TierMax, TierPathological}

// Case is one materialized test case.
//
// It carries metadata only. The bytes live on disk: a 1e5-line case is ~1 MB
// and must never enter the model's context, which is exactly the constraint
// that makes generated code (rather than generated text) the only viable way to
// produce large inputs.
type Case struct {
	Name      string `json:"name"`
	Seed      int64  `json:"seed"`
	N         int    `json:"n"`
	Shape     string `json:"shape"`
	Tier      Tier   `json:"tier"`
	Bytes     int64  `json:"bytes"`
	Lines     int    `json:"lines"`
	SHA256    string `json:"sha256"`
	FirstLine string `json:"first_line"`
	LastLine  string `json:"last_line"`
	Score     int    `json:"score,omitempty"`
	// Path is filled by the workspace and stripped before the model sees it.
	Path string `json:"-"`
}

// Preview renders the bounded view of a case that is safe to show the model.
func (c Case) Preview() string {
	return fmt.Sprintf("name=%s tier=%s n=%d shape=%s bytes=%d lines=%d sha256=%s first=%q last=%q",
		c.Name, c.Tier, c.N, c.Shape, c.Bytes, c.Lines, c.SHA256[:min(12, len(c.SHA256))], c.FirstLine, c.LastLine)
}

// Artifact is a compiled program in the workspace.
type Artifact string

// The artifacts a complete problem needs.
//
// Validator and the extra solutions come from the Polygon pipeline: a validator
// is what makes "n up to 1e5" enforceable, and solutions with declared expected
// verdicts are what prove the tests have teeth.
const (
	ArtifactGenerator Artifact = "generator"
	ArtifactSolution  Artifact = "solution"
	ArtifactBrute     Artifact = "brute"
	ArtifactSlow      Artifact = "slow"
	ArtifactWrong     Artifact = "wrong"
	ArtifactValidator Artifact = "validator"
)

// AllArtifacts lists every artifact in reporting order.
var AllArtifacts = []Artifact{
	ArtifactGenerator, ArtifactSolution, ArtifactBrute,
	ArtifactSlow, ArtifactWrong, ArtifactValidator,
}

// Expected verdicts for a solution artifact, Polygon-style. Declaring the
// expectation is what turns "we ran some code" into a real test.
type Verdict string

const (
	VerdictAC Verdict = "ac"
	VerdictTL Verdict = "tl"
	VerdictWA Verdict = "wa"
	VerdictRE Verdict = "re"
)

// Sample is a statement example.
type Sample struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation,omitempty"`
}

// Statement is the human-facing part of the problem.
type Statement struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	InputFormat  string `json:"input_format"`
	OutputFormat string `json:"output_format"`
	// Constraints is the human-readable LaTeX bounds block, rendered as its own
	// section on the problem page. It is kept separate from Description so the
	// bounds are always presented as constraints (Codeforces style) rather than
	// buried in prose.
	Constraints string   `json:"constraints"`
	Hint        string   `json:"hint,omitempty"`
	Tags        []string `json:"tags"`
	Difficulty  string   `json:"difficulty"`
	Samples     []Sample `json:"samples"`
}

// Check is one gate result.
type Check struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
	// Warning marks a passed check with a caveat (e.g. limits too tight).
	Warning bool `json:"warning,omitempty"`
}

// Report is the outcome of finalize.
type Report struct {
	Ready   bool    `json:"ready"`
	Checks  []Check `json:"checks"`
	Cases   []Case  `json:"cases"`
	Blocked []string
}

// Failed lists the names of failed checks.
func (r Report) Failed() []string {
	out := make([]string, 0, len(r.Checks))
	for _, c := range r.Checks {
		if !c.Passed {
			out = append(out, c.Name)
		}
	}
	return out
}

// Manifest is the durable record of what the harness produced, stored on the
// job and copied to problem_generation_assets on apply.
type Manifest struct {
	Constraints Constraints `json:"constraints"`
	Limits      Limits      `json:"limits"`
	Statement   Statement   `json:"statement"`
	Cases       []Case      `json:"cases"`
	Report      Report      `json:"report"`
	// Artifacts maps each artifact to the source the model wrote, so the
	// problem can be regenerated and audited later.
	Artifacts map[string]string `json:"artifacts"`
	Languages map[string]string `json:"languages"`
	Usage     Usage             `json:"usage"`
}

// Usage accounts for what the run consumed inside the sandbox.
type Usage struct {
	SandboxCPU   time.Duration `json:"sandbox_cpu"`
	BytesWritten int64         `json:"bytes_written"`
	Compilations int           `json:"compilations"`
	CaseRuns     int           `json:"case_runs"`
	Materialized int           `json:"cases_materialized"`
	// Turns is the length of the model conversation, reported separately from
	// Materialized because the two are unrelated (a 16-turn run can materialize
	// 31 cases) and turn_count is what operators read to judge loop efficiency.
	ReferenceRuns int `json:"reference_runs"`
	Turns         int `json:"turns"`
}

// hashBytes returns the manifest hash of a case body.
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
