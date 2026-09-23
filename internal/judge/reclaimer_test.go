package judge

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
)

type fakeSubStore struct {
	mu        sync.Mutex
	judgingAt map[string]time.Time
	pending   []string
	staleErr  error
	listErr   error
}

func newFakeSubStore() *fakeSubStore {
	return &fakeSubStore{judgingAt: map[string]time.Time{}}
}

func (f *fakeSubStore) judging(id string, since time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.judgingAt[id] = since
}

func (f *fakeSubStore) RequeueStale(_ context.Context, olderThan time.Duration) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.staleErr != nil {
		return nil, f.staleErr
	}
	cutoff := time.Now().Add(-olderThan)
	var ids []string
	for id, started := range f.judgingAt {
		if started.Before(cutoff) {
			ids = append(ids, id)
			delete(f.judgingAt, id)
			f.pending = append(f.pending, id)
		}
	}
	return ids, nil
}

func (f *fakeSubStore) ListPending(_ context.Context, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	if limit > len(f.pending) {
		limit = len(f.pending)
	}
	return append([]string(nil), f.pending[:limit]...), nil
}

func (f *fakeSubStore) Create(context.Context, *model.Submission) error { return nil }
func (f *fakeSubStore) GetByID(context.Context, string) (*model.Submission, error) {
	return nil, nil
}
func (f *fakeSubStore) ListByProblem(context.Context, string, int, int) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fakeSubStore) ListByUser(context.Context, string, int, int, string, string) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fakeSubStore) ListPublicByUser(context.Context, string, int, int) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fakeSubStore) ListByContest(context.Context, string, int, int, model.SubmissionFilter) ([]model.Submission, int, error) {
	return nil, 0, nil
}
func (f *fakeSubStore) UpdateStatus(context.Context, string, model.SubmissionStatus) {}
func (f *fakeSubStore) UpdateResult(context.Context, string, model.SubmissionStatus, int, int, int, string, []model.TestCaseResult) error {
	return nil
}
func (f *fakeSubStore) ClaimPending(context.Context, string, string) (bool, error) {
	return false, nil
}
func (f *fakeSubStore) UpdateResultClaimed(context.Context, string, string, model.SubmissionStatus, int, int, int, string, []model.TestCaseResult) (bool, error) {
	return false, nil
}
func (f *fakeSubStore) UpdateRemoteID(context.Context, string, string, string) error { return nil }
func (f *fakeSubStore) UpdateBotID(context.Context, string, string, string) error    { return nil }
func (f *fakeSubStore) GetPendingRemoteSubmissions(context.Context) ([]model.PendingRemoteSubmission, error) {
	return nil, nil
}
func (f *fakeSubStore) GetUnsubmittedRemoteSubmissions(context.Context) ([]model.Submission, error) {
	return nil, nil
}
func (f *fakeSubStore) GetProblemStats(context.Context, string) (*model.ProblemStats, error) {
	return nil, nil
}
func (f *fakeSubStore) GetUserStats(context.Context, string) (*model.UserProblemStats, error) {
	return nil, nil
}
func (f *fakeSubStore) GetPlatformStats(context.Context) (*model.PlatformStats, error) {
	return nil, nil
}
func (f *fakeSubStore) GetHackableSubmissions(context.Context, string, string) ([]model.Submission, error) {
	return nil, nil
}

type failOnceQueue struct {
	*queue.MemoryQueue
	failNext bool
}

func (q *failOnceQueue) Enqueue(ctx context.Context, id string, priority int) error {
	if q.failNext {
		q.failNext = false
		return errors.New("queue unavailable")
	}
	return q.MemoryQueue.Enqueue(ctx, id, priority)
}

func drain(q *queue.MemoryQueue) []string {
	var got []string
	for q.Len() > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		id, err := q.Dequeue(ctx)
		cancel()
		if err != nil {
			break
		}
		got = append(got, id)
	}
	return got
}

func TestReclaimer_RequeuesStaleAndPendingRows(t *testing.T) {
	store := newFakeSubStore()
	store.judging("sub-stale", time.Now().Add(-30*time.Minute))
	store.pending = append(store.pending, "sub-pending")
	q := queue.NewMemory()
	r := NewReclaimer(store, q, 15*time.Minute, time.Minute)

	if n := r.Sweep(context.Background()); n != 2 {
		t.Fatalf("first sweep: want 2 queued, got %d", n)
	}
	got := drain(q)
	if len(got) != 2 || !contains(got, "sub-stale") || !contains(got, "sub-pending") {
		t.Fatalf("queue after sweep = %v", got)
	}
}

func TestReclaimer_RetriesPendingSubmissionAfterEnqueueFailure(t *testing.T) {
	store := newFakeSubStore()
	store.pending = append(store.pending, "sub-pending")
	q := &failOnceQueue{MemoryQueue: queue.NewMemory(), failNext: true}
	r := NewReclaimer(store, q, 15*time.Minute, time.Minute)

	if n := r.Sweep(context.Background()); n != 0 {
		t.Fatalf("first sweep: want 0 queued after enqueue failure, got %d", n)
	}
	if n := r.Sweep(context.Background()); n != 1 {
		t.Fatalf("second sweep: want 1 queued, got %d", n)
	}
	if got := drain(q.MemoryQueue); len(got) != 1 || got[0] != "sub-pending" {
		t.Fatalf("queue after retry = %v", got)
	}
}

func TestReclaimer_LeavesLiveClaimAlone(t *testing.T) {
	store := newFakeSubStore()
	store.judging("sub-live", time.Now().Add(-time.Minute))
	q := queue.NewMemory()
	r := NewReclaimer(store, q, 15*time.Minute, time.Minute)

	if n := r.Sweep(context.Background()); n != 0 {
		t.Fatalf("live claim: want 0 queued, got %d", n)
	}
	if got := drain(q); len(got) != 0 {
		t.Fatalf("live claim must not be queued, got %v", got)
	}
}

func TestReclaimer_ZeroStaleAfterUsesSafeDefault(t *testing.T) {
	store := newFakeSubStore()
	store.judging("sub-live", time.Now().Add(-time.Second))
	q := queue.NewMemory()
	r := NewReclaimer(store, q, 0, 0)

	if n := r.Sweep(context.Background()); n != 0 {
		t.Fatalf("zero config must protect a live judgment, got %d", n)
	}
}

func TestReclaimer_StoreErrorsAreNonFatal(t *testing.T) {
	for name, configure := range map[string]func(*fakeSubStore){
		"stale sweep":  func(s *fakeSubStore) { s.staleErr = errors.New("db down") },
		"list pending": func(s *fakeSubStore) { s.listErr = errors.New("db down") },
	} {
		t.Run(name, func(t *testing.T) {
			store := newFakeSubStore()
			configure(store)
			q := queue.NewMemory()
			r := NewReclaimer(store, q, time.Minute, time.Minute)
			if n := r.Sweep(context.Background()); n != 0 {
				t.Fatalf("store error: want 0 queued, got %d", n)
			}
			if got := drain(q); len(got) != 0 {
				t.Fatalf("store error must not queue work, got %v", got)
			}
		})
	}
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
