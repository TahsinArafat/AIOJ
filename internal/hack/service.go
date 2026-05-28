package hack

import (
	"context"
	"errors"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

var ErrActiveHackPhaseNotEnabled = errors.New("hack phase is not active")

type Service struct {
	hackStore    store.HackStore
	contestStore store.ContestStore
	subStore     store.SubmissionStore
}

func NewService(hs store.HackStore, cs store.ContestStore, ss store.SubmissionStore) *Service {
	return &Service{
		hackStore:    hs,
		contestStore: cs,
		subStore:     ss,
	}
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

	h := &model.Hack{
		ContestID:    req.ContestID,
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

	// Mocking sandbox judge result for now. Hacking is successful if input length is odd (for testing).
	success := len(req.TestInput)%2 != 0
	status := "failure"
	if success {
		status = "success"
	}

	s.hackStore.UpdateStatus(ctx, h.ID, status, success)

	return &model.HackResult{
		HackID:         h.ID,
		Status:         status,
		Success:        success,
		ExpectedOutput: "expected_mock_output",
		ActualOutput:   "actual_mock_output",
	}, nil
}
