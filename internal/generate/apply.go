package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ApplyRequest struct {
	Sections          []string   `json:"sections"`
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at,omitempty"`
}
type ApplyResult struct {
	ProblemID   string `json:"problem_id"`
	ProblemSlug string `json:"problem_slug"`
}
type ApplyService struct {
	Jobs         store.AIGenerationStore
	Problems     generateProblemStore
	Editorials   store.EditorialStore
	Assets       GenerationAssetStore
	Audit        store.AuditStore
	TestdataRoot string
	StagingRoot  string
}
type GenerationAssetStore interface {
	Create(context.Context, *model.ProblemGenerationAsset) error
}

func (s *ApplyService) Apply(ctx context.Context, job *model.AIGenerationJob, actorID string, req ApplyRequest) (*ApplyResult, error) {
	if job.Status != "ready" {
		return nil, fmt.Errorf("job is not ready")
	}
	var p Proposal
	if json.Unmarshal(job.Proposal, &p) != nil {
		return nil, fmt.Errorf("proposal is invalid")
	}
	// The brief carries the setter's target rating; the proposal only describes
	// the authored content. A job without a readable brief is not fatal: the
	// problem simply stays unrated.
	var brief Request
	_ = json.Unmarshal(job.NormalizedBrief, &brief)
	selected := map[string]bool{}
	for _, v := range req.Sections {
		selected[v] = true
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("select at least one section")
	}
	var problem *model.Problem
	var err error
	if job.ProblemID != nil {
		problem, err = s.Problems.GetByID(ctx, *job.ProblemID)
		if err != nil || problem == nil {
			return nil, fmt.Errorf("problem not found")
		}
		if req.ExpectedUpdatedAt != nil && !problem.UpdatedAt.Equal(*req.ExpectedUpdatedAt) {
			return nil, fmt.Errorf("problem changed since generation")
		}
		if !s.Problems.HasAccess(ctx, problem.ID, actorID, "owner", "co_author") {
			return nil, fmt.Errorf("problem edit access required")
		}
	} else {
		problem = &model.Problem{ID: uuid.NewString(), Slug: slugify(p.Title), Visible: false, Source: "ai", AIGenerated: true, CreatedBy: actorID, CheckerType: "lines", ScoringMode: "complete", SubtaskAggregation: "min"}
	}
	if selected["statement"] {
		problem.Title = p.Title
		problem.Description = p.Description
		problem.Constraints = p.Constraints
		problem.InputFormat = p.InputFormat
		problem.OutputFormat = p.OutputFormat
		problem.Hint = p.Hint
		problem.SampleCases = p.SampleCases
		// Default rather than store zero: this judge uses time_limit as the CPU
		// limit, so a proposal that omitted it would produce an unjudgeable
		// problem that TLEs everything.
		problem.TimeLimit = defaultInt(p.TimeLimitMs, 2000)
		problem.MemoryLimit = problemMemoryLimitKB(p)
		problem.Difficulty = p.Difficulty
		problem.Tags = p.Tags
		// The harness never invents a rating, it aims at the one on the brief.
		// Apply it here, or every AI-created problem lands unrated and drops out
		// of the problem list's rating filter.
		if brief.Rating > 0 {
			problem.Rating = brief.Rating
		}
	}
	isNew := job.ProblemID == nil
	if isNew {
		if err = s.Problems.Create(ctx, problem); err != nil {
			return nil, err
		}
	} else if err = s.Problems.Update(ctx, problem.ID, problem); err != nil {
		return nil, err
	}
	// Only publish an official editorial when the proposal actually contains
	// one. The harness pipeline authors a statement, solution and tests but no
	// editorial, and the UI selects every section by default, so an unguarded
	// upsert created a blank "official" editorial on the problem. The reference
	// solution is still preserved in the generation asset written below.
	if (selected["editorial"] || selected["solution"]) &&
		(p.EditorialTitle != "" || p.EditorialContent != "") {
		e := &model.Editorial{ProblemID: problem.ID, UserID: actorID, Title: p.EditorialTitle, Content: p.EditorialContent, SolutionCode: p.SolutionCode, SolutionLanguage: p.SolutionLanguage, Approach: p.EditorialApproach, TimeComplexity: p.TimeComplexity, SpaceComplexity: p.SpaceComplexity, IsOfficial: true}
		if err = s.Editorials.UpsertOfficial(ctx, e); err != nil {
			return nil, err
		}
	}
	if selected["tests"] && s.Assets != nil {
		if s.TestdataRoot == "" {
			return nil, fmt.Errorf("testdata root is not configured")
		}
		versionPath := filepath.Join(s.TestdataRoot, problem.ID, "ai", job.ID)
		stagedPath := filepath.Join(s.StagingRoot, job.ID)
		if s.StagingRoot == "" {
			return nil, fmt.Errorf("AI staging root is not configured")
		}
		if err := os.MkdirAll(filepath.Dir(versionPath), 0o750); err != nil {
			return nil, fmt.Errorf("create testdata parent: %w", err)
		}
		if err := os.Rename(stagedPath, versionPath); err != nil {
			return nil, fmt.Errorf("promote staged testdata: %w", err)
		}
		problem.TestdataPath = versionPath
		problem.TestCaseScore = discoverCaseScores(versionPath)
		// Carry the validator onto the problem itself, not just onto the asset.
		// Migration 000083 added problems.validator_language/source so a
		// generated validator would survive the job, but nothing copied it
		// across: the setter's "validate cases" endpoint reads the problem row,
		// so every AI-created problem answered "this problem has no validator"
		// while its validator sat in problem_generation_assets.
		problem.ValidatorLanguage = p.ValidatorLanguage
		problem.ValidatorSource = p.ValidatorCode
		if err = s.Problems.Update(ctx, problem.ID, problem); err != nil {
			return nil, err
		}
		seeds, _ := json.Marshal(p.Seeds)
		report := job.ValidationReport
		a := &model.ProblemGenerationAsset{ProblemID: problem.ID, JobID: job.ID, GeneratorLanguage: p.GeneratorLanguage, GeneratorSource: p.GeneratorCode, ReferenceSolutionLanguage: p.SolutionLanguage, ReferenceSolutionSource: p.SolutionCode, ValidatorLanguage: p.ValidatorLanguage, ValidatorSource: p.ValidatorCode, SeedManifest: seeds, ValidationSummary: report}
		if err = s.Assets.Create(ctx, a); err != nil {
			return nil, err
		}
	}
	if err = s.Jobs.MarkApplied(ctx, job.ID); err != nil {
		return nil, err
	}
	if s.Audit != nil {
		detail, _ := json.Marshal(map[string]any{"job_id": job.ID, "problem_id": problem.ID, "sections": req.Sections})
		_ = s.Audit.Insert(ctx, &model.AuditEntry{ActorID: actorID, Action: "AI APPLY", TargetType: "problem", TargetID: problem.ID, Detail: detail})
	}
	return &ApplyResult{ProblemID: problem.ID, ProblemSlug: problem.Slug}, nil
}

// discoverCaseScores pairs every `<stem>.in` with its `<stem>.out` in dir and
// spreads 100 points across the pairs.
//
// Pairs are sorted by name first: os.ReadDir already returns entries sorted, but
// the remainder from the 100/n split has to land on a stable case or the same
// test data would score differently on every apply.
func discoverCaseScores(dir string) []model.TestCaseScore {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var cases []model.TestCaseScore
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".in") {
			continue
		}
		stem := strings.TrimSuffix(name, ".in")
		if _, err := os.Stat(filepath.Join(dir, stem+".out")); err == nil {
			cases = append(cases, model.TestCaseScore{InputName: name, OutputName: stem + ".out"})
		}
	}
	// Spread 100 points across the cases. A flat 10 per case meant a 12-case
	// problem was scored out of 120, which breaks every percentage and
	// partial-score display downstream.
	assignScores(cases)
	return cases
}

// defaultInt returns v when positive, otherwise fallback.
func defaultInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

// defaultMemoryLimitKB is the judge's conventional 256 MB when a proposal
// omits the limit.
const defaultMemoryLimitKB = 262144

// problemMemoryLimitKB resolves the value written to problems.memory_limit.
//
// That column is in KILOBYTES: the admin create handler stores the form value
// verbatim, the judge reads it as KB and multiplies by 1024 itself
// (judge.WorkerPool.getEffectiveLimits), and the problem page renders
// memory_limit/1024 as MB. Keep this a straight passthrough — multiplying here
// (as this file once did) stores bytes, which displays as "262144000 MB" and
// hands the sandbox a limit 1024x larger than intended.
func problemMemoryLimitKB(p Proposal) int {
	return defaultInt(p.MemoryLimitKB, defaultMemoryLimitKB)
}

// assignScores distributes 100 points across the cases, giving the remainder to
// the earliest cases so the total is exactly 100.
//
// A flat 10 points per case meant a 12-case problem was scored out of 120,
// which breaks every percentage and partial-score display downstream.
func assignScores(cases []model.TestCaseScore) {
	if len(cases) == 0 {
		return
	}
	base := 100 / len(cases)
	remainder := 100 % len(cases)
	for i := range cases {
		cases[i].Score = base
		if i < remainder {
			cases[i].Score++
		}
	}
}

// generateProblemStore is the problem surface the apply step touches: read the
// target, check the caller may write it, create it, and update it.
type generateProblemStore interface {
	GetByID(ctx context.Context, id string) (*model.Problem, error)
	Create(ctx context.Context, p *model.Problem) error
	Update(ctx context.Context, id string, p *model.Problem) error
	HasAccess(ctx context.Context, problemID, userID string, requiredLevels ...string) bool
}
