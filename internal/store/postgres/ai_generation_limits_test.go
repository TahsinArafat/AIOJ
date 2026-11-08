package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// makeQueuedJob inserts one job for actorID and returns its id.
func makeQueuedJob(t *testing.T, ctx context.Context, store *AIGenerationStore, actorID string) string {
	t.Helper()
	empty := json.RawMessage(`{}`)
	job := &model.AIGenerationJob{
		ActorID:            actorID,
		ArtifactType:       "full_problem",
		SubmittedBrief:     empty,
		NormalizedBrief:    empty,
		ContextSnapshot:    empty,
		ConnectionSnapshot: empty,
		PromptVersion:      "problem-v2",
		PromptHash:         "test",
	}
	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return job.ID
}

func newTestActor(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	users := NewUserStore(db)
	actorID := uuid.NewString()
	if err := users.Create(ctx, &model.User{
		ID: actorID, Username: "gen_" + uuid.NewString()[:8],
		Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "setter",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM users WHERE id=$1", actorID)
	})
	return actorID
}

// A job the actor asked to cancel must stop holding a concurrency slot as soon
// as the request lands.
//
// Cancelling a running job only sets cancel_requested_at; status stays 'running'
// until the worker observes it. Counting on status alone therefore kept the slot
// taken, and an immediate retry was rejected with "limit reached" right after a
// successful cancel — the error appeared to be about nothing.
func TestCountActiveByActorIgnoresCancelledJobs(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	store := NewAIGenerationStore(db)
	actorID := newTestActor(t, ctx, db)

	// A running job holds the slot.
	jobID := makeQueuedJob(t, ctx, store, actorID)
	if _, err := db.ExecContext(ctx, `UPDATE ai_generation_jobs SET status='running',stage='drafting' WHERE id=$1`, jobID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	running, queued, err := store.CountActiveByActor(ctx, actorID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if running != 1 || queued != 0 {
		t.Fatalf("running=%d queued=%d, want 1/0 while the job is live", running, queued)
	}

	// Asking to cancel frees the slot even though the worker has not caught up
	// and the status is still 'running'.
	if _, err := db.ExecContext(ctx, `UPDATE ai_generation_jobs SET cancel_requested_at=NOW() WHERE id=$1`, jobID); err != nil {
		t.Fatalf("request cancel: %v", err)
	}
	running, queued, err = store.CountActiveByActor(ctx, actorID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if running != 0 || queued != 0 {
		t.Fatalf("running=%d queued=%d, want 0/0 after a cancel request", running, queued)
	}
}

// The limit message names the job that is in the way, so it must be the running
// one rather than an older queued job.
func TestActiveJobByActorPrefersTheRunningJob(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	store := NewAIGenerationStore(db)
	actorID := newTestActor(t, ctx, db)

	queuedID := makeQueuedJob(t, ctx, store, actorID)
	runningID := makeQueuedJob(t, ctx, store, actorID)
	if _, err := db.ExecContext(ctx, `UPDATE ai_generation_jobs SET status='running' WHERE id=$1`, runningID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	blocking, err := store.ActiveJobByActor(ctx, actorID)
	if err != nil {
		t.Fatalf("active job: %v", err)
	}
	if blocking == nil {
		t.Fatal("expected a blocking job")
	}
	if blocking.ID != runningID {
		t.Fatalf("blocking job = %s, want the running one %s (queued was %s)", blocking.ID, runningID, queuedID)
	}

	// A cancelled job is not blocking anything.
	if _, err := db.ExecContext(ctx, `UPDATE ai_generation_jobs SET cancel_requested_at=NOW() WHERE id=$1`, runningID); err != nil {
		t.Fatalf("request cancel: %v", err)
	}
	blocking, err = store.ActiveJobByActor(ctx, actorID)
	if err != nil {
		t.Fatalf("active job: %v", err)
	}
	if blocking == nil || blocking.ID != queuedID {
		t.Fatalf("blocking job = %v, want the queued one %s", blocking, queuedID)
	}
}
