package judge

import (
	"context"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/judge/runner"
	"github.com/tahsinarafat/aioj/internal/model"
)

// failingRunner reports the context error, standing in for a judgment that was
// interrupted mid-flight.
type failingRunner struct{}

func (failingRunner) Run(ctx context.Context, _ runner.RunInput) (runner.RunOutput, error) {
	if err := ctx.Err(); err != nil {
		return runner.RunOutput{}, err
	}
	return runner.RunOutput{}, context.Canceled
}

// TestStopWaitsForInFlightWork covers the shutdown path: the pool used to
// abandon running judgments, and the worker then wrote a System Error verdict
// over a submission that was about to be judged correctly.
func TestStopWaitsForInFlightWork(t *testing.T) {
	pool := &WorkerPool{}
	released := make(chan struct{})

	pool.wg.Add(1)
	go func() {
		defer pool.wg.Done()
		<-released
	}()

	stopped := make(chan struct{})
	go func() {
		pool.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while a judgment was still running")
	case <-time.After(50 * time.Millisecond):
		// Correct: Stop is blocked on the in-flight work.
	}

	close(released)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after the in-flight work finished")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	pool := &WorkerPool{}
	pool.Stop()
	pool.Stop() // must not panic or block
}

// TestInterruptedJudgmentIsLeftForReclaimer: a cancelled context means shutdown,
// not a broken submission, so no verdict may be written over it.
func TestInterruptedJudgmentIsLeftForReclaimer(t *testing.T) {
	store := &fencingStore{}
	pool := &WorkerPool{subStore: store}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pool.runAndFinalize(ctx, &model.Submission{ID: "sub-1"}, "claim-token",
		&model.Problem{ID: "prob-1"}, failingRunner{}, runner.RunInput{}, "")

	if store.applied {
		t.Fatal("an interrupted judgment must not be persisted as a verdict")
	}
}
