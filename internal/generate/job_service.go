package generate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"time"
)

type JobService struct {
	Jobs     store.AIGenerationJobStore
	Models   store.AIModelStore
	Problems problemReadAccess
}
type CreateJobRequest struct {
	ProblemID    *string
	AIModelID    *string
	ArtifactType string
	Brief        Request
	// ActorIsAdmin lets an admin target any problem. The read-side endpoints
	// (Get, ListProblem) already grant admins this bypass; without the same rule
	// on Create an admin could open AI Studio on a problem they do not own and
	// be refused with "problem edit access required" before any work started.
	ActorIsAdmin bool
}

// Concurrency limits per actor. The harness holds a judge slot and a provider
// budget for the length of a run, so only one job may run at a time; the queue
// is bounded so one setter cannot monopolise the worker.
const (
	maxRunningPerActor = 1
	maxQueuedPerActor  = 3
)

func (s *JobService) Create(ctx context.Context, actorID string, req CreateJobRequest) (*model.AIGenerationJob, error) {
	normalized, err := req.Brief.Normalize()
	if err != nil {
		return nil, err
	}
	if normalized.ProblemType != "standard" {
		return nil, fmt.Errorf("v1 supports standard batch problems only")
	}
	switch req.ArtifactType {
	case "full_problem", "statement", "tests", "solution", "editorial":
	default:
		return nil, fmt.Errorf("unsupported artifact_type")
	}
	running, queued, err := s.Jobs.CountActiveByActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if running >= maxRunningPerActor || queued >= maxQueuedPerActor {
		blocking, _ := s.Jobs.ActiveJobByActor(ctx, actorID)
		return nil, errors.New(limitMessage(running, queued, blocking))
	}
	if req.ProblemID != nil {
		p, err := s.Problems.GetByID(ctx, *req.ProblemID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, fmt.Errorf("problem not found")
		}
		if !req.ActorIsAdmin && !s.Problems.HasAccess(ctx, p.ID, actorID, "owner", "co_author") {
			return nil, fmt.Errorf("problem edit access required")
		}
	}
	submitted, _ := json.Marshal(req.Brief)
	norm, _ := json.Marshal(normalized)
	hash := sha256.Sum256([]byte(systemPrompt + profileInstructions(normalized)))
	job := &model.AIGenerationJob{ActorID: actorID, ProblemID: req.ProblemID, AIModelID: req.AIModelID, ArtifactType: req.ArtifactType, SubmittedBrief: submitted, NormalizedBrief: norm, ContextSnapshot: json.RawMessage(`{}`), ConnectionSnapshot: json.RawMessage(`{}`), PromptVersion: "problem-v2", PromptHash: hex.EncodeToString(hash[:])}
	if err := s.Jobs.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// limitMessage explains which limit was hit and, when known, which job is in the
// way, so the setter can act instead of guessing. A job started from a different
// problem blocks a new one and nothing else in this flow surfaces it.
//
// Pure so it can be tested without a store.
func limitMessage(running, queued int, blocking *model.AIGenerationJob) string {
	if running >= maxRunningPerActor {
		if blocking != nil {
			return fmt.Sprintf("you already have a generation running (%s, stage %s). "+
				"Wait for it to finish or cancel it, then start this one", blocking.ID, blocking.Stage)
		}
		return "you already have a generation running. Wait for it to finish or cancel it, then start this one"
	}
	return fmt.Sprintf("you already have %d generations queued (limit %d). "+
		"Wait for one to run or cancel one", queued, maxQueuedPerActor)
}

type JobWorker struct {
	Jobs    store.AIGenerationJobStore
	Models  store.AIModelStore
	Poll    time.Duration
	Timeout time.Duration
	Sandbox *SandboxValidator
	// CancelPoll is how often a running job checks the store for a cancel
	// request. Zero means one second.
	CancelPoll time.Duration
}

func (w *JobWorker) Run(ctx context.Context) {
	if w.Poll <= 0 {
		w.Poll = time.Second
	}
	if w.Timeout <= 0 {
		w.Timeout = 5 * time.Minute
	}
	// Exponential backoff: start at Poll (default 1 s), double on each empty
	// cycle up to 30 s, reset immediately when a job is found.  Avoids a
	// continuous DB round-trip every second when the AI feature is enabled but
	// no generation jobs are queued.
	const maxPoll = 30 * time.Second
	interval := w.Poll
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			_, _ = w.Jobs.RequeueStale(ctx, 10*time.Minute)
			if w.processOne(ctx) {
				interval = w.Poll // found work — reset to minimum
			} else if interval < maxPoll {
				interval *= 2
				if interval > maxPoll {
					interval = maxPoll
				}
			}
		}
	}
}

// watchCancel stops a run as soon as the setter asks to cancel it. The flag
// lives in the database, so it is polled; the goroutine exits when the run
// finishes (done), the worker shuts down (ctx), or a cancel is found.
func (w *JobWorker) watchCancel(ctx context.Context, jobID string, cancel context.CancelFunc, done <-chan struct{}) {
	interval := w.CancelPoll
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			if cancelled, err := w.Jobs.IsCancellationRequested(ctx, jobID); err == nil && cancelled {
				cancel()
				return
			}
		}
	}
}

func (w *JobWorker) processOne(ctx context.Context) bool {
	token := uuid.NewString()
	job, err := w.Jobs.ClaimNext(ctx, token)
	if err != nil || job == nil {
		return false
	}
	if cancelled, _ := w.Jobs.IsCancellationRequested(ctx, job.ID); cancelled {
		_ = w.Jobs.MarkCancelled(ctx, job.ID, token)
		return true
	}
	var brief Request
	if json.Unmarshal(job.NormalizedBrief, &brief) != nil {
		_ = w.Jobs.Fail(ctx, job.ID, token, "invalid_brief", "stored brief is invalid", nil)
		return true
	}
	m, err := w.Models.GetDefault(ctx)
	if job.AIModelID != nil {
		m, err = w.Models.GetByID(ctx, *job.AIModelID)
	}
	if err != nil || m == nil {
		_ = w.Jobs.Fail(ctx, job.ID, token, "connection_unavailable", "AI connection is unavailable", nil)
		return true
	}
	_ = w.Jobs.Heartbeat(ctx, job.ID, token, "drafting", 15)
	client := ai.NewWithOptions(m.Endpoint, m.APIKey, m.ModelName, ai.ClientOptions{Mode: m.StructuredOutputMode, Timeout: time.Duration(m.RequestTimeoutSeconds) * time.Second, MaxOutputTokens: m.MaxOutputTokens, AllowPrivateNetwork: m.AllowPrivateNetwork})
	runCtx, cancel := context.WithTimeout(ctx, w.Timeout)
	defer cancel()
	// A cancel request only flips a column; without watching it here the run
	// kept going until the provider answered or the timeout expired.
	watchDone := make(chan struct{})
	defer close(watchDone)
	go w.watchCancel(ctx, job.ID, cancel, watchDone)
	start := time.Now()
	result, err := client.CompleteJSON(runCtx, systemPrompt, buildUserPrompt(brief), ProposalJSONSchema())
	if err != nil {
		if cancelled, _ := w.Jobs.IsCancellationRequested(ctx, job.ID); cancelled {
			_ = w.Jobs.MarkCancelled(ctx, job.ID, token)
			return true
		}
		_ = w.Jobs.Fail(ctx, job.ID, token, "provider_error", "AI generation failed", nil)
		return true
	}
	if cancelled, _ := w.Jobs.IsCancellationRequested(ctx, job.ID); cancelled {
		_ = w.Jobs.MarkCancelled(ctx, job.ID, token)
		return true
	}
	_ = w.Jobs.Heartbeat(ctx, job.ID, token, "validating", 75)
	proposal, report, err := ParseAndValidateProposal(result.Content, job.ArtifactType)
	reportJSON, _ := json.Marshal(report)
	if err != nil {
		_ = w.Jobs.Fail(ctx, job.ID, token, "validation_failed", "Generated proposal failed validation", reportJSON)
		return true
	}
	if (job.ArtifactType == "full_problem" || job.ArtifactType == "tests") && w.Sandbox != nil {
		sandboxChecks, sandboxErr := w.Sandbox.Validate(runCtx, proposal, job.ID)
		report.Checks = append(report.Checks, sandboxChecks...)
		if sandboxErr != nil {
			if cancelled, _ := w.Jobs.IsCancellationRequested(ctx, job.ID); cancelled {
				_ = w.Jobs.MarkCancelled(ctx, job.ID, token)
				return true
			}
			report.Valid = false
			report.Checks = append(report.Checks, ValidationCheck{Name: "sandbox", Passed: false, Message: sandboxErr.Error()})
			reportJSON, _ = json.Marshal(report)
			_ = w.Jobs.Fail(ctx, job.ID, token, "sandbox_validation_failed", "Generated code failed sandbox validation", reportJSON)
			return true
		}
		reportJSON, _ = json.Marshal(report)
	}
	proposalJSON, _ := json.Marshal(proposal)
	_ = w.Jobs.Complete(ctx, job.ID, token, proposalJSON, reportJSON, result.PromptTokens, result.CompletionTokens, int(time.Since(start).Milliseconds()))
	return true
}

// problemReadAccess is what the job service asks of a problem: fetch it, and
// check the caller may read it.
type problemReadAccess interface {
	GetByID(ctx context.Context, id string) (*model.Problem, error)
	HasAccess(ctx context.Context, problemID, userID string, requiredLevels ...string) bool
}
