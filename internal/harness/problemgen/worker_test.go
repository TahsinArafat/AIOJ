package problemgen

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/harness"
	"github.com/tahsinarafat/aioj/internal/model"
)

// fakeJobStore scripts the queue for one job.
type fakeJobStore struct {
	job      *model.AIGenerationJob
	claimed  bool
	manifest json.RawMessage
	proposal json.RawMessage
	turns    int
	ready    bool
	failed   string

	// cancelled and markedCancelled are read by the cancel watcher and the test
	// concurrently, so they are atomic rather than plain fields.
	cancelled       atomic.Bool
	markedCancelled atomic.Bool
}

func (f *fakeJobStore) ClaimNext(context.Context, string) (*model.AIGenerationJob, error) {
	if f.claimed {
		return nil, nil
	}
	f.claimed = true
	return f.job, nil
}

func (f *fakeJobStore) Heartbeat(context.Context, string, string, string, int) error { return nil }

func (f *fakeJobStore) IsCancellationRequested(context.Context, string) (bool, error) {
	return f.cancelled.Load(), nil
}

func (f *fakeJobStore) MarkCancelled(context.Context, string, string) error {
	f.markedCancelled.Store(true)
	return nil
}

func (f *fakeJobStore) RequeueStale(context.Context, time.Duration) ([]string, error) {
	return nil, nil
}

func (f *fakeJobStore) SetManifest(_ context.Context, _, _ string, proposal, manifest, _ json.RawMessage, ready bool, turns int) error {
	f.proposal = proposal
	f.manifest = manifest
	f.ready = ready
	f.turns = turns
	return nil
}

func (f *fakeJobStore) Fail(_ context.Context, _, _, code, message string, _ json.RawMessage) error {
	f.failed = code + ": " + message
	return nil
}

func briefJSON(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(Brief{Tags: []string{"implementation"}, Rating: 1200})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newTestWorker(t *testing.T, store *fakeJobStore, model harness.Model) *Worker {
	t.Helper()
	return &Worker{
		Jobs:       store,
		Runner:     &fakeRunner{},
		Transcript: harness.NewMemoryTranscript(),
		Staging:    t.TempDir(),
		MaxBytes:   8 << 20,
		Resolve: func(context.Context, *string) (harness.Model, error) {
			return model, nil
		},
	}
}

func TestWorkerRunsAJobAndStoresTheManifest(t *testing.T) {
	store := &fakeJobStore{job: &model.AIGenerationJob{ID: "job-1", NormalizedBrief: briefJSON(t)}}
	w := newTestWorker(t, store, &toolScriptModel{script: happyScript()})

	w.processOne(context.Background())

	if store.failed != "" {
		t.Fatalf("job failed: %s", store.failed)
	}
	if store.manifest == nil {
		t.Fatal("no manifest stored")
	}
	if !store.ready {
		t.Fatal("a complete run must be marked ready")
	}

	var m Manifest
	if err := json.Unmarshal(store.manifest, &m); err != nil {
		t.Fatalf("stored manifest is not valid JSON: %v", err)
	}
	if m.Statement.Title == "" {
		t.Fatal("manifest has no statement")
	}
	if len(m.Cases) == 0 {
		t.Fatal("manifest has no cases")
	}
}

func TestWorkerFailsAJobWithAnUnusableBrief(t *testing.T) {
	store := &fakeJobStore{job: &model.AIGenerationJob{ID: "job-2", NormalizedBrief: json.RawMessage(`{`)}}
	w := newTestWorker(t, store, &toolScriptModel{})

	w.processOne(context.Background())

	if store.failed == "" {
		t.Fatal("a malformed brief must fail the job")
	}
	if store.manifest != nil {
		t.Fatal("no manifest should be stored for a malformed brief")
	}
}

func TestWorkerFailsAJobWhenNoModelIsAvailable(t *testing.T) {
	store := &fakeJobStore{job: &model.AIGenerationJob{ID: "job-3", NormalizedBrief: briefJSON(t)}}
	w := newTestWorker(t, store, &toolScriptModel{})
	w.Resolve = func(context.Context, *string) (harness.Model, error) { return nil, nil }

	w.processOne(context.Background())

	if store.failed == "" {
		t.Fatal("a job with no model must fail rather than hang")
	}
}

func TestWorkerHonoursCancellationBeforeStarting(t *testing.T) {
	store := &fakeJobStore{job: &model.AIGenerationJob{ID: "job-4", NormalizedBrief: briefJSON(t)}}
	store.cancelled.Store(true)
	w := newTestWorker(t, store, &toolScriptModel{script: happyScript()})

	w.processOne(context.Background())

	if store.manifest != nil {
		t.Fatal("a cancelled job must not produce a manifest")
	}
	if store.failed != "" {
		t.Fatal("cancellation is not a failure")
	}
}

// blockingModel holds the first turn open until the run context is cancelled,
// which is what a long provider call looks like to the worker.
type blockingModel struct {
	started chan struct{}
	once    sync.Once
}

func (m *blockingModel) CompleteWithTools(ctx context.Context, _ []ai.Message, _ []ai.Tool) (*ai.CompletionResult, error) {
	m.once.Do(func() { close(m.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestWorkerStopsARunningJobWhenCancelled(t *testing.T) {
	store := &fakeJobStore{job: &model.AIGenerationJob{ID: "job-5", NormalizedBrief: briefJSON(t)}}
	model := &blockingModel{started: make(chan struct{})}
	w := newTestWorker(t, store, model)
	w.CancelPoll = 5 * time.Millisecond

	// The setter clicks Cancel while the run is in flight.
	go func() {
		<-model.started
		store.cancelled.Store(true)
	}()

	done := make(chan struct{})
	go func() {
		w.processOne(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("processOne did not return after the cancel request — the run ignored it")
	}

	if store.failed != "" {
		t.Fatalf("cancellation must not be recorded as a failure: %s", store.failed)
	}
	if !store.markedCancelled.Load() {
		t.Fatal("the job must be marked cancelled, not left running")
	}
	if store.manifest != nil {
		t.Fatal("a cancelled job must not store a manifest")
	}
}

// fakeProblems loads one problem, or nothing.
type fakeProblems struct {
	problem *model.Problem
}

func (f *fakeProblems) GetByID(context.Context, string) (*model.Problem, error) {
	return f.problem, nil
}

// A job that targets an existing problem must not run without it: generating
// blind is what made "improve this problem" overwrite it with an unrelated one.
func TestWorkerFailsAJobWhenTheProblemCannotBeLoaded(t *testing.T) {
	problemID := "p-1"
	store := &fakeJobStore{job: &model.AIGenerationJob{ID: "job-6", ProblemID: &problemID, NormalizedBrief: briefJSON(t)}}
	w := newTestWorker(t, store, &toolScriptModel{script: happyScript()})
	w.Problems = &fakeProblems{problem: nil}

	w.processOne(context.Background())

	if !strings.Contains(store.failed, "problem_unavailable") {
		t.Fatalf("failed = %q, want problem_unavailable", store.failed)
	}
	if store.manifest != nil {
		t.Fatal("a job without its problem must not produce a manifest")
	}
}

// The existing problem must reach the model, not just the job row.
func TestWorkerFeedsTheExistingProblemToTheModel(t *testing.T) {
	problemID := "p-2"
	store := &fakeJobStore{job: &model.AIGenerationJob{
		ID: "job-7", ProblemID: &problemID, ArtifactType: "statement", NormalizedBrief: briefJSON(t),
	}}
	script := &toolScriptModel{script: happyScript()}
	w := newTestWorker(t, store, script)
	w.Problems = &fakeProblems{problem: &model.Problem{ID: problemID, Title: "Existing Echo", Description: "The original statement."}}

	w.processOne(context.Background())

	if store.failed != "" {
		t.Fatalf("job failed: %s", store.failed)
	}
	if len(script.prompts) == 0 {
		t.Fatal("the model was never called")
	}
	prompt := script.prompts[0]
	for _, want := range []string{"Existing Echo", "The original statement.", "Rewrite the statement"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestWorkerRejectsAnIncompleteConfiguration(t *testing.T) {
	if err := (&Worker{}).Ready(); err == nil {
		t.Fatal("expected an error with no job store")
	}
	store := &fakeJobStore{}
	if err := (&Worker{Jobs: store}).Ready(); err == nil {
		t.Fatal("expected an error with no resolver")
	}
	if err := (&Worker{Jobs: store, Resolve: func(context.Context, *string) (harness.Model, error) { return nil, nil }}).Ready(); err == nil {
		t.Fatal("expected an error with no runner")
	}
}

func TestBriefRenderCarriesOperatorIntentOnly(t *testing.T) {
	b := Brief{Tags: []string{"dp"}, Rating: 1700, FormatProfile: "codeforces", SolverMode: "solo", AdditionalPrompt: "grids"}
	out := b.RenderFor("full_problem", nil)
	for _, want := range []string{"dp", "1700", "codeforces", "solo", "grids"} {
		if !contains(out, want) {
			t.Fatalf("rendered brief missing %q:\n%s", want, out)
		}
	}
	// The brief must not be able to weaken the gates.
	for _, forbidden := range []string{"skip", "ignore the gate", "without validation"} {
		if contains(out, forbidden) {
			t.Fatalf("rendered brief contains %q", forbidden)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
