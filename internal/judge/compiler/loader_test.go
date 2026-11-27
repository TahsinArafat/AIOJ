package compiler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// stubSource is a ConfigSource whose answer the test controls.
type stubSource struct {
	docs  map[string]json.RawMessage
	err   error
	calls int
}

func (s *stubSource) ListLanguageConfigs(context.Context) (map[string]json.RawMessage, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.docs, nil
}

func doc(t *testing.T, cfg LangConfig) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// The loader replaced a sync.Once that was never invalidated, which is why an
// admin edit reached the submit dropdown but not the judge. A read after
// Invalidate must see the new config.
func TestLoaderInvalidateDropsTheCache(t *testing.T) {
	src := &stubSource{docs: map[string]json.RawMessage{
		"cpp": doc(t, LangConfig{Key: "cpp", Name: "C++", CompileCmd: "g++ -O2"}),
	}}
	l := NewLoader(src)

	first, err := l.All(context.Background())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if first["cpp"].Name != "C++" {
		t.Fatalf("name = %q, want C++", first["cpp"].Name)
	}

	// An admin save lands, and invalidates.
	src.docs["cpp"] = doc(t, LangConfig{Key: "cpp", Name: "C++ 20", CompileCmd: "g++ -std=c++20"})
	l.Invalidate()

	after, err := l.All(context.Background())
	if err != nil {
		t.Fatalf("All after invalidate: %v", err)
	}
	if after["cpp"].Name != "C++ 20" {
		t.Errorf("name = %q after invalidate, want \"C++ 20\" — the cache is not being cleared", after["cpp"].Name)
	}
	if after["cpp"].CompileCmd != "g++ -std=c++20" {
		t.Errorf("compile = %q, want the edited command", after["cpp"].CompileCmd)
	}
}

// Within the TTL the loader must not re-read: it sits on the per-submission
// path, and a database read there is the cost this cache exists to avoid.
func TestLoaderReusesWithinTTL(t *testing.T) {
	src := &stubSource{docs: map[string]json.RawMessage{
		"cpp": doc(t, LangConfig{Key: "cpp", Name: "C++"}),
	}}
	l := NewLoader(src)

	for i := 0; i < 5; i++ {
		if _, err := l.All(context.Background()); err != nil {
			t.Fatalf("All: %v", err)
		}
	}
	if src.calls != 1 {
		t.Errorf("source read %d times within the TTL, want 1", src.calls)
	}
}

// Past the TTL the loader re-reads, which is how a change made in another
// process — an admin save in the backend, seen by the judge worker — arrives.
func TestLoaderRefreshesAfterTTL(t *testing.T) {
	src := &stubSource{docs: map[string]json.RawMessage{
		"cpp": doc(t, LangConfig{Key: "cpp", Name: "C++"}),
	}}
	l := NewLoader(src)

	now := time.Now()
	l.now = func() time.Time { return now }

	if _, err := l.All(context.Background()); err != nil {
		t.Fatalf("All: %v", err)
	}

	src.docs["cpp"] = doc(t, LangConfig{Key: "cpp", Name: "C++ 20"})
	now = now.Add(DefaultTTL + time.Second)

	got, err := l.All(context.Background())
	if err != nil {
		t.Fatalf("All after TTL: %v", err)
	}
	if got["cpp"].Name != "C++ 20" {
		t.Errorf("name = %q after TTL, want the refreshed \"C++ 20\"", got["cpp"].Name)
	}
}

// An unreachable registry must not fail every submission: a stale config beats a
// judge that stops judging. The error is only returned when there is nothing to
// fall back to.
func TestLoaderServesStaleConfigWhenTheSourceFails(t *testing.T) {
	src := &stubSource{docs: map[string]json.RawMessage{
		"cpp": doc(t, LangConfig{Key: "cpp", Name: "C++"}),
	}}
	l := NewLoader(src)

	now := time.Now()
	l.now = func() time.Time { return now }

	if _, err := l.All(context.Background()); err != nil {
		t.Fatalf("All: %v", err)
	}

	src.err = errors.New("database is down")
	now = now.Add(DefaultTTL + time.Second)

	got, err := l.All(context.Background())
	if err != nil {
		t.Fatalf("All with a failing source and a warm cache returned an error: %v", err)
	}
	if got["cpp"] == nil {
		t.Fatal("the cached config was dropped when the source failed")
	}
}

// With no cache and no source there is nothing honest to serve, so the failure
// has to surface rather than becoming an empty language list.
func TestLoaderFailsWhenSourceFailsAndNothingIsCached(t *testing.T) {
	l := NewLoader(&stubSource{err: errors.New("database is down")})
	if _, err := l.All(context.Background()); err == nil {
		t.Fatal("expected an error when the source fails with a cold cache")
	}
}

// One malformed row must not hide every other language: it is dropped, which
// surfaces as "unsupported language" for that key alone.
func TestLoaderSkipsMalformedConfigs(t *testing.T) {
	src := &stubSource{docs: map[string]json.RawMessage{
		"cpp":    doc(t, LangConfig{Key: "cpp", Name: "C++"}),
		"broken": json.RawMessage(`{"name": 42}`),
	}}
	l := NewLoader(src)

	got, err := l.All(context.Background())
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if got["cpp"] == nil {
		t.Error("a valid config was dropped because a sibling was malformed")
	}
	if _, ok := got["broken"]; ok {
		t.Error("a malformed config was accepted")
	}
}

// Get is the per-submission path; it must agree with All.
func TestLoaderGet(t *testing.T) {
	src := &stubSource{docs: map[string]json.RawMessage{
		"cpp": doc(t, LangConfig{Key: "cpp", Name: "C++"}),
	}}
	l := NewLoader(src)

	cfg, ok := l.Get(context.Background(), "cpp")
	if !ok || cfg == nil || cfg.Name != "C++" {
		t.Fatalf("Get(cpp) = %+v, %v", cfg, ok)
	}
	if _, ok := l.Get(context.Background(), "nope"); ok {
		t.Error("Get returned a language that does not exist")
	}
}
