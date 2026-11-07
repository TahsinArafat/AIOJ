package problemgen

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/harness"
	"github.com/tahsinarafat/aioj/internal/model"
)

// fakeJobStore scripts the queue for one job.
type fakeJobStore struct {
	job       *model.AIGenerationJob
	claimed   bool
	manifest  json.RawMessage
	ready     bool
	failed    string
	cancelled bool
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
	return f.cancelled, nil
}

func (f *fakeJobStore) MarkCancelled(context.Context, string, string) error { return nil }

func (f *fakeJobStore) RequeueStale(context.Context, time.Duration) ([]string, error) {
	return nil, nil
}

func (f *fakeJobStore) SetManifest(_ context.Context, _, _ string, manifest, _ json.RawMessage, ready bool, _ int) error {
	f.manifest = manifest
	f.ready = ready
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
	store := &fakeJobStore{
		job:       &model.AIGenerationJob{ID: "job-4", NormalizedBrief: briefJSON(t)},
		cancelled: true,
	}
	w := newTestWorker(t, store, &toolScriptModel{script: happyScript()})

	w.processOne(context.Background())

	if store.manifest != nil {
		t.Fatal("a cancelled job must not produce a manifest")
	}
	if store.failed != "" {
		t.Fatal("cancellation is not a failure")
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
	out := b.Render()
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
