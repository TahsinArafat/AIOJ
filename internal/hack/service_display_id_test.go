package hack

import (
	"context"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type canonicalHackContestStore struct {
	contest *model.Contest
}

func (s canonicalHackContestStore) GetByID(_ context.Context, ref string) (*model.Contest, error) {
	if ref == "12" {
		return s.contest, nil
	}
	return nil, nil
}

func (canonicalHackContestStore) Create(context.Context, *model.Contest) error { return nil }
func (canonicalHackContestStore) GetBySlug(context.Context, string) (*model.Contest, error) {
	return nil, nil
}
func (canonicalHackContestStore) List(context.Context, int, int) ([]model.Contest, int, error) {
	return nil, 0, nil
}
func (canonicalHackContestStore) ListByCreatedBy(context.Context, string, int, int) ([]model.Contest, int, error) {
	return nil, 0, nil
}
func (canonicalHackContestStore) Update(context.Context, *model.Contest) error { return nil }
func (canonicalHackContestStore) Delete(context.Context, string) error         { return nil }
func (canonicalHackContestStore) AddProblem(context.Context, string, string, string, int, int) error {
	return nil
}
func (canonicalHackContestStore) UpdateProblem(context.Context, string, string, string, int, int) error {
	return nil
}
func (canonicalHackContestStore) RemoveProblem(context.Context, string, string) error {
	return nil
}
func (canonicalHackContestStore) GetProblems(context.Context, string) ([]model.ContestProblem, error) {
	return nil, nil
}
func (canonicalHackContestStore) GetContestProblemByIndex(context.Context, string, string) (*model.Problem, error) {
	return nil, nil
}
func (canonicalHackContestStore) GetParticipants(context.Context, string) ([]string, error) {
	return nil, nil
}
func (canonicalHackContestStore) IsParticipant(context.Context, string, string) (bool, error) {
	return false, nil
}
func (canonicalHackContestStore) GetUsername(context.Context, string) string { return "" }
func (canonicalHackContestStore) AddPermission(context.Context, string, string, string) error {
	return nil
}
func (canonicalHackContestStore) RemovePermission(context.Context, string, string) error {
	return nil
}
func (canonicalHackContestStore) GetPermissions(context.Context, string) ([]model.ContestPermission, error) {
	return nil, nil
}
func (canonicalHackContestStore) HasAccess(context.Context, string, string, ...string) bool {
	return true
}
func (canonicalHackContestStore) CheckGroupRestriction(context.Context, string, string) (bool, error) {
	return true, nil
}

type canonicalHackStore struct {
	created *model.Hack
}

func (s *canonicalHackStore) Create(_ context.Context, h *model.Hack) error {
	h.ID = "hack-1"
	s.created = h
	return nil
}
func (s *canonicalHackStore) GetByID(context.Context, string) (*model.Hack, error) {
	return nil, nil
}
func (s *canonicalHackStore) UpdateStatus(context.Context, string, string, bool) error { return nil }
func (s *canonicalHackStore) GetByContest(context.Context, string) ([]model.Hack, error) {
	return nil, nil
}
func (s *canonicalHackStore) GetHackableSubmissions(context.Context, string, string) ([]model.Submission, error) {
	return nil, nil
}

type canonicalSubmissionStore struct {
	store.SubmissionStore
	sub *model.Submission
}

func (s canonicalSubmissionStore) GetByID(context.Context, string) (*model.Submission, error) {
	return s.sub, nil
}

func TestSubmitHackPersistsCanonicalContestUUID(t *testing.T) {
	contest := &model.Contest{ID: "00000000-0000-0000-0000-000000000012", HackPhaseEnabled: true}
	hackStore := &canonicalHackStore{}
	svc := NewService(
		hackStore,
		canonicalHackContestStore{contest: contest},
		canonicalSubmissionStore{sub: &model.Submission{ID: "sub-1", UserID: "defender"}},
	)

	result, err := svc.SubmitHack(context.Background(), "attacker", model.HackRequest{
		ContestID:    "12",
		ProblemID:    "problem-1",
		SubmissionID: "sub-1",
		TestInput:    "1 2",
	})
	if err != nil {
		t.Fatalf("SubmitHack returned error: %v", err)
	}
	if result.HackID != "hack-1" {
		t.Fatalf("HackID = %q, want hack-1", result.HackID)
	}
	if hackStore.created == nil || hackStore.created.ContestID != contest.ID {
		t.Fatalf("persisted contest ID = %#v, want canonical %q", hackStore.created, contest.ID)
	}
}

var _ store.HackStore = (*canonicalHackStore)(nil)
var _ store.ContestStore = canonicalHackContestStore{}
