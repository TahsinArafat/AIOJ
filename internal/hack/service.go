package hack

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

var ErrActiveHackPhaseNotEnabled = errors.New("hack phase is not active")

// ErrHackJudgingUnavailable is returned when the judge the hack needs to run
// against is not configured. Hacks are refused rather than auto-accepted: a
// stub that resolved hacks without running code made the whole hack phase
// decorative.
var ErrHackJudgingUnavailable = errors.New("hack judging is not available on this deployment")

// Runner executes a defender's submission against a hacker's input.
// The production implementation is *Judge; tests supply a fake.
type Runner interface {
	Enabled() bool
	Run(ctx context.Context, prob *model.Problem, sub *model.Submission, testInput, expectedOutput string) (Verdict, error)
}

// contestLookup is the one contest query this service makes: it needs the round
// a hack belongs to, and nothing else. Narrow on the consumer side so a test
// fake is one method, not the whole ContestStore.
type contestLookup interface {
	GetByID(ctx context.Context, id string) (*model.Contest, error)
}

// submissionLookup is the one submission query a hack needs: the row being
// hacked, so it can check the owner and the contest. Not the whole submission
// store — a hack never lists, rejudges or voids anything.
type submissionLookup interface {
	GetByID(ctx context.Context, id string) (*model.Submission, error)
}

type Service struct {
	hackStore    store.HackStore
	contestStore contestLookup
	subStore     submissionLookup
	probStore    problemLookup
	judge        Runner
}

func NewService(hs store.HackStore, cs contestLookup, ss submissionLookup) *Service {
	return &Service{
		hackStore:    hs,
		contestStore: cs,
		subStore:     ss,
	}
}

// WithJudge attaches the real judge that executes the defender's submission.
// Without it SubmitHack refuses with ErrHackJudgingUnavailable.
func (s *Service) WithJudge(probStore store.ProblemStore, judge Runner) *Service {
	s.probStore = probStore
	s.judge = judge
	return s
}

func (s *Service) SubmitHack(ctx context.Context, hackerID string, req model.HackRequest) (*model.HackResult, error) {
	sub, err := s.subStore.GetByID(ctx, req.SubmissionID)
	if err != nil || sub == nil {
		return nil, errors.New("submission not found")
	}

	if sub.UserID == hackerID {
		return nil, errors.New("cannot hack your own submission")
	}

	contest, err := s.contestStore.GetByID(ctx, req.ContestID)
	if err != nil || contest == nil {
		return nil, errors.New("contest not found")
	}

	if !contest.HackPhaseEnabled {
		return nil, ErrActiveHackPhaseNotEnabled
	}

	now := time.Now()
	if contest.HackPhaseStart != nil && now.Before(*contest.HackPhaseStart) {
		return nil, ErrActiveHackPhaseNotEnabled
	}
	if contest.HackPhaseEnd != nil && now.After(*contest.HackPhaseEnd) {
		return nil, ErrActiveHackPhaseNotEnabled
	}

	if s.judge == nil || !s.judge.Enabled() || s.probStore == nil {
		return nil, ErrHackJudgingUnavailable
	}
	prob, err := s.probStore.GetByID(ctx, sub.ProblemID)
	if err != nil || prob == nil {
		return nil, errors.New("problem not found")
	}

	h := &model.Hack{
		ContestID:    contest.ID,
		ProblemID:    req.ProblemID,
		HackerID:     hackerID,
		DefenderID:   sub.UserID,
		SubmissionID: req.SubmissionID,
		TestInput:    req.TestInput,
		Status:       "pending",
	}
	if err := s.hackStore.Create(ctx, h); err != nil {
		return nil, err
	}

	verdict, err := s.judge.Run(ctx, prob, sub, req.TestInput, req.ExpectedOutput)
	if err != nil {
		slog.Error("hack judging failed", "hack", h.ID, "submission", sub.ID, "error", err)
		_ = s.hackStore.UpdateStatus(ctx, h.ID, "error", false)
		return nil, errors.New("hack could not be judged: " + err.Error())
	}

	status := "failure"
	if verdict.Failed {
		status = "success"
	}
	if err := s.hackStore.UpdateStatus(ctx, h.ID, status, verdict.Failed); err != nil {
		slog.Error("persist hack verdict failed", "hack", h.ID, "error", err)
	}

	return &model.HackResult{
		HackID:         h.ID,
		Status:         status,
		Success:        verdict.Failed,
		ExpectedOutput: req.ExpectedOutput,
		ActualOutput:   verdict.ActualOutput,
		Message:        verdict.Detail,
	}, nil
}

// problemLookup is the one problem query a hack needs: the problem a submission
// belongs to, so the judge can run it. Already declared in the handler package
// for the same shape; repeated here because hack is a separate package and the
// interfaces are deliberately consumer-local.
type problemLookup interface {
	GetByID(ctx context.Context, id string) (*model.Problem, error)
}
