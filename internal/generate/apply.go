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
	Problems     store.ProblemStore
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
		problem.InputFormat = p.InputFormat
		problem.OutputFormat = p.OutputFormat
		problem.Hint = p.Hint
		problem.SampleCases = p.SampleCases
		// Default rather than store zero: this judge uses time_limit as the CPU
		// limit, so a proposal that omitted it would produce an unjudgeable
		// problem that TLEs everything.
		problem.TimeLimit = defaultInt(p.TimeLimitMs, 2000)
		problem.MemoryLimit = defaultInt(p.MemoryLimitKB, 262144) * 1024
		problem.Difficulty = p.Difficulty
		problem.Tags = p.Tags
	}
	isNew := job.ProblemID == nil
	if isNew {
		if err = s.Problems.Create(ctx, problem); err != nil {
			return nil, err
		}
	} else if err = s.Problems.Update(ctx, problem.ID, problem); err != nil {
		return nil, err
	}
	if selected["editorial"] || selected["solution"] {
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
		problem.TestCaseScore = nil
		entries, err := os.ReadDir(versionPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".in") {
				stem := strings.TrimSuffix(entry.Name(), ".in")
				if _, err := os.Stat(filepath.Join(versionPath, stem+".out")); err == nil {
					problem.TestCaseScore = append(problem.TestCaseScore, model.TestCaseScore{InputName: entry.Name(), OutputName: stem + ".out"})
				}
			}
		}
		// Spread 100 points across the cases. A flat 10 per case meant a
		// 12-case problem was scored out of 120, which breaks every percentage
		// and partial-score display downstream.
		assignScores(problem.TestCaseScore)
		if err = s.Problems.Update(ctx, problem.ID, problem); err != nil {
			return nil, err
		}
		seeds, _ := json.Marshal(p.Seeds)
		report := job.ValidationReport
		a := &model.ProblemGenerationAsset{ProblemID: problem.ID, JobID: job.ID, GeneratorLanguage: p.GeneratorLanguage, GeneratorSource: p.GeneratorCode, ReferenceSolutionLanguage: p.SolutionLanguage, ReferenceSolutionSource: p.SolutionCode, SeedManifest: seeds, ValidationSummary: report}
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

// defaultInt returns v when positive, otherwise fallback.
func defaultInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
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
