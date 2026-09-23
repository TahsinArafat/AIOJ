package judge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// fencingStore is the public SubmissionStore behavior needed by WorkerPool.
// It records the token used for the final write and can report a lost claim.
type fencingStore struct {
	claimed bool
	token   string
	applied bool
	err     error
}

func (f *fencingStore) ClaimPending(_ context.Context, _, token string) (bool, error) {
	f.claimed = true
	f.token = token
	return true, nil
}
func (f *fencingStore) UpdateResultClaimed(_ context.Context, _, token string, _ model.SubmissionStatus, _, _, _ int, _ string, _ []model.TestCaseResult) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.applied = token == f.token
	return f.applied, nil
}
func (f *fencingStore) GetByID(context.Context, string) (*model.Submission, error) {
	return nil, errors.New("stop after claim")
}
func (f *fencingStore) Create(context.Context, *model.Submission) error { return nil }
func (f *fencingStore) ListByProblem(context.Context, string, int, int) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fencingStore) ListByUser(context.Context, string, int, int, string, string) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fencingStore) ListPublicByUser(context.Context, string, int, int) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fencingStore) ListByContest(context.Context, string, int, int, model.SubmissionFilter) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fencingStore) UpdateStatus(context.Context, string, model.SubmissionStatus) {}
func (f *fencingStore) UpdateResult(context.Context, string, model.SubmissionStatus, int, int, int, string, []model.TestCaseResult) error {
	return nil
}
func (f *fencingStore) RequeueStale(context.Context, time.Duration) ([]string, error) {
	return nil, nil
}
func (f *fencingStore) UpdateRemoteID(context.Context, string, string, string) error { return nil }
func (f *fencingStore) UpdateBotID(context.Context, string, string, string) error    { return nil }
func (f *fencingStore) GetPendingRemoteSubmissions(context.Context) ([]model.PendingRemoteSubmission, error) {
	return nil, nil
}
func (f *fencingStore) GetUnsubmittedRemoteSubmissions(context.Context) ([]model.Submission, error) {
	return nil, nil
}
func (f *fencingStore) ListPending(context.Context, int) ([]string, error) { return nil, nil }
func (f *fencingStore) GetProblemStats(context.Context, string) (*model.ProblemStats, error) {
	return nil, nil
}
func (f *fencingStore) GetUserStats(context.Context, string) (*model.UserProblemStats, error) {
	return nil, nil
}
func (f *fencingStore) GetPlatformStats(context.Context) (*model.PlatformStats, error) {
	return nil, nil
}
func (f *fencingStore) GetHackableSubmissions(context.Context, string, string) ([]model.Submission, error) {
	return nil, nil
}

func TestWorkerProcessUsesClaimTokenForTerminalWrite(t *testing.T) {
	store := &fencingStore{}
	pool := &WorkerPool{subStore: store}
	pool.process(context.Background(), "submission-1")

	if !store.claimed {
		t.Fatal("worker did not claim the submission")
	}
	if store.token == "" {
		t.Fatal("worker did not generate a claim token")
	}
	if !store.applied {
		t.Fatal("terminal result was not written with the active claim token")
	}
}
