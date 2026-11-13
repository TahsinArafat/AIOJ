package judge

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/judge/runner"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * A wedged judgment must cost one slot for a bounded time, not forever.
 *
 * The reported failure was the judge containers going silent — no compiling, no
 * test cases running, the IDE down — and a container restart fixing it. The
 * mechanism: nothing bounded a judgment end to end. Each sandbox call had its own
 * 60s HTTP timeout, but a batch runs one call per test case, so a slow sandbox
 * multiplied that by the case count while the job sat inside one of the pool's
 * few slots. Once every slot held a parked job the judge was dead while both
 * containers still looked healthy, and the reclaimer's 15-minute sweep was the
 * only thing that would eventually clear it.
 *
 * These tests pin the three ways that can end, because collapsing them is the
 * easy mistake: an expired deadline must fail the submission, a plain error must
 * fail it with that error, and a shutdown must NOT fail it at all.
 */

// recordingStore embeds the fencing store's full SubmissionStore surface and
// records what the worker tried to persist.
type recordingStore struct {
	*fencingStore
	mu        sync.Mutex
	writes    int
	lastStat  model.SubmissionStatus
	lastText  string
	writeDone chan struct{}
}

func newRecordingStore() *recordingStore {
	return &recordingStore{fencingStore: &fencingStore{}, writeDone: make(chan struct{})}
}

func (r *recordingStore) UpdateResultClaimed(_ context.Context, _, token string, status model.SubmissionStatus, _, _, _ int, text string, _ []model.TestCaseResult) (bool, error) {
	r.mu.Lock()
	r.writes++
	r.lastStat = status
	r.lastText = text
	tokenOK := token == r.fencingStore.token
	r.mu.Unlock()
	select {
	case <-r.writeDone:
	default:
		close(r.writeDone)
	}
	return tokenOK, nil
}

func (r *recordingStore) snapshot() (int, model.SubmissionStatus, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writes, r.lastStat, r.lastText
}

// blockingRunner simulates a sandbox that never answers until its context ends,
// which is what a wedged go-judge call looks like from the worker's side.
type blockingRunner struct{}

func (blockingRunner) Run(ctx context.Context, _ runner.RunInput) (runner.RunOutput, error) {
	<-ctx.Done()
	return runner.RunOutput{}, ctx.Err()
}

// errorRunner is an ordinary failure: the sandbox answered, with a fault.
type errorRunner struct{}

func (errorRunner) Run(context.Context, runner.RunInput) (runner.RunOutput, error) {
	return runner.RunOutput{}, errors.New("sandbox exploded")
}

func testProblem() *model.Problem {
	return &model.Problem{ID: "p1", TimeLimit: 1000, MemoryLimit: 262144}
}

func testSubmission() *model.Submission {
	return &model.Submission{ID: "s1", ProblemID: "p1", Language: "cpp"}
}

// The reported bug, at its root: a judgment that outlives its deadline is failed
// and its slot released, rather than holding the slot until a human restarts the
// container.
//
// The deadline is armed here directly rather than by process(), which is the only
// caller that does. TestProcessArmsTheDeadline pins that wiring separately; what
// this covers is the dispatch — given an expired judgment, the right verdict is
// written and the call returns.
func TestWedgedJudgmentIsFailedAndItsSlotReleased(t *testing.T) {
	rec := newRecordingStore()
	// A real flow claims the row first, which is what sets the fencing token the
	// terminal write must match. Reproducing that here means the verdict is
	// genuinely applied rather than merely attempted, so the test proves the
	// submission actually leaves 'judging'.
	rec.fencingStore.token = "tok"
	pool := &WorkerPool{subStore: rec, health: NewHealthState(), judgmentTimeout: 40 * time.Millisecond}

	// What process() hands to judge(): a context bounded by the judgment timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		pool.runAndFinalize(ctx, testSubmission(), "tok", testProblem(),
			blockingRunner{}, runner.RunInput{}, "")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a judgment with an expired deadline never returned; the slot would stay claimed forever")
	}

	writes, status, text := rec.snapshot()
	if writes != 1 {
		t.Fatalf("terminal writes = %d, want exactly 1 — the verdict must be persisted so the submission leaves 'judging'", writes)
	}
	if status != model.StatusSE {
		t.Fatalf("status = %q, want %q for a judgment that ran out of time", status, model.StatusSE)
	}
	if text == "" {
		t.Fatal("the verdict carried no explanation; an admin would see a bare System Error with nothing to act on")
	}
	t.Logf("recorded verdict: %s — %q", status, text)
}

// An ordinary sandbox fault is still reported as itself. The deadline must not
// swallow the real error or relabel it as a timeout.
func TestOrdinaryFailureKeepsItsOwnError(t *testing.T) {
	rec := newRecordingStore()
	rec.fencingStore.token = "tok"
	pool := &WorkerPool{subStore: rec, health: NewHealthState(), judgmentTimeout: time.Minute}

	pool.runAndFinalize(context.Background(), testSubmission(), "tok", testProblem(),
		errorRunner{}, runner.RunInput{}, "")

	writes, status, text := rec.snapshot()
	if writes != 1 || status != model.StatusSE {
		t.Fatalf("writes=%d status=%q, want 1 write with %q", writes, status, model.StatusSE)
	}
	if text != "sandbox exploded" {
		t.Fatalf("detail = %q, want the runner's own error unchanged", text)
	}
}

// The case that must NOT write: a shutdown provides no verdict. Recording one
// would overwrite a submission that the reclaimer is about to re-run, which is
// the pre-existing behaviour this change must not reintroduce.
func TestShutdownDoesNotWriteAVerdict(t *testing.T) {
	rec := newRecordingStore()
	pool := &WorkerPool{subStore: rec, health: NewHealthState(), judgmentTimeout: time.Minute}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		pool.runAndFinalize(ctx, testSubmission(), "tok", testProblem(),
			blockingRunner{}, runner.RunInput{}, "")
	}()

	// Let the judgment start, then shut down while it is in flight.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runAndFinalize did not return after shutdown")
	}

	if writes, status, _ := rec.snapshot(); writes != 0 {
		t.Fatalf("a shutdown wrote %d verdict(s) (last=%q); it must leave the row claimed so the reclaimer re-runs it", writes, status)
	}
}

// deadlineObservingStore records the deadline on the context the worker passes
// down, so the test can prove process() bounded the judgment at all.
type deadlineObservingStore struct {
	*fencingStore
	deadline time.Time
	hasOne   bool
}

func (d *deadlineObservingStore) GetByID(ctx context.Context, _ string) (*model.Submission, error) {
	d.deadline, d.hasOne = ctx.Deadline()
	return nil, errors.New("stop here: the deadline is already recorded")
}

// Without this, process() could stop arming the deadline and the tests above
// would still pass, because they arm their own context — which is exactly the
// shape of "a policy test proves the rule, a gate test proves the request is
// subject to it". The unbounded judgment was the bug, so the arming is the
// load-bearing part and gets its own assertion.
func TestProcessArmsTheDeadline(t *testing.T) {
	store := &deadlineObservingStore{fencingStore: &fencingStore{}}
	pool := &WorkerPool{subStore: store, health: NewHealthState(), judgmentTimeout: 10 * time.Minute}

	pool.process(context.Background(), "s1")

	if !store.hasOne {
		t.Fatal("process ran the judgment with an unbounded context; a wedged sandbox would hold the slot indefinitely")
	}
	remaining := time.Until(store.deadline)
	// Generous bounds: it must be finite and near what was configured, not an
	// accidental default and not an already-expired deadline.
	if remaining <= 0 {
		t.Fatalf("the deadline was already expired on entry (%v from now); every judgment would fail instantly", remaining)
	}
	if remaining > 10*time.Minute+time.Second {
		t.Fatalf("deadline is %v away, want ~10m — the configured timeout was not applied", remaining)
	}
	t.Logf("judgment deadline armed %v from entry", remaining.Round(time.Second))
}

// A structurally-built pool — as the pool's own tests do — leaves the field zero.
// WithTimeout(ctx, 0) is already expired, so without the fallback every judgment
// would fail instantly with a timeout nobody configured.
func TestZeroTimeoutFieldFallsBackToTheDefault(t *testing.T) {
	store := &deadlineObservingStore{fencingStore: &fencingStore{}}
	pool := &WorkerPool{subStore: store, health: NewHealthState()}

	pool.process(context.Background(), "s1")

	if !store.hasOne {
		t.Fatal("no deadline armed")
	}
	if remaining := time.Until(store.deadline); remaining < time.Minute {
		t.Fatalf("deadline %v away for a zero-valued field; want the %v default, not an immediate expiry",
			remaining, DefaultJudgmentTimeout)
	}
}
