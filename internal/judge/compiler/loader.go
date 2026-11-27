package compiler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ConfigSource supplies the live language configs, keyed by language key. Each
// value is a JSON document in the LangConfig shape.
//
// It is an interface rather than the Postgres store so the loader — and every
// consumer of it — can be tested without a database.
type ConfigSource interface {
	ListLanguageConfigs(ctx context.Context) (map[string]json.RawMessage, error)
}

// DefaultTTL is how long a loaded language set is reused.
//
// It bounds how stale a config can be. Admin writes call Invalidate on the
// loader that served them, so the backend process is immediate. The judge-worker
// is a SEPARATE process and cannot be signalled from here, so TTL is the
// worst-case propagation delay for a compile command change — 15 s is chosen to
// make that invisible in practice while keeping a database read off the
// per-submission path.
const DefaultTTL = 15 * time.Second

// Loader is the single cache of live language configs.
//
// Before this existed each consumer held its own sync.Once over the lang/*.yaml
// directory: WorkerPool, SubmissionHandler and the validator closure each had
// one, none was ever invalidated, and all of them assumed a restart was needed
// to see a change. That assumption is why an admin edit appeared in the submit
// dropdown (PublicLanguageHandler re-read per request) but was rejected by the
// judge ("unsupported language") until the next deploy. One cache with a TTL and
// an explicit Invalidate makes the two agree.
type Loader struct {
	source ConfigSource
	ttl    time.Duration
	now    func() time.Time

	mu       sync.RWMutex
	langs    map[string]*LangConfig
	loadedAt time.Time
	loaded   bool
}

func NewLoader(source ConfigSource) *Loader {
	return &Loader{source: source, ttl: DefaultTTL, now: time.Now}
}

// SetTTL overrides the cache lifetime. Tests use it to make staleness
// deterministic; production keeps DefaultTTL.
func (l *Loader) SetTTL(ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ttl = ttl
	l.loaded = false
}

// Invalidate drops the cache so the next read reloads it. Called after an admin
// write so the change takes effect in this process at once.
func (l *Loader) Invalidate() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loaded = false
	l.langs = nil
}

// All returns every language, keyed by key.
//
// A load failure returns the last good set when there is one: an unreachable
// database must not turn every submission into a failure, and a config that is a
// few seconds stale is far better than a judge that stops judging. The error is
// returned only when there is nothing cached to fall back to.
func (l *Loader) All(ctx context.Context) (map[string]*LangConfig, error) {
	l.mu.RLock()
	if l.loaded && l.now().Sub(l.loadedAt) < l.ttl {
		langs := l.langs
		l.mu.RUnlock()
		return langs, nil
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	// Another goroutine may have refreshed while we waited for the write lock.
	if l.loaded && l.now().Sub(l.loadedAt) < l.ttl {
		return l.langs, nil
	}

	raw, err := l.source.ListLanguageConfigs(ctx)
	if err != nil {
		if l.loaded && l.langs != nil {
			slog.Warn("compiler: language refresh failed, serving the cached set",
				"error", err, "count", len(l.langs))
			return l.langs, nil
		}
		return nil, fmt.Errorf("load language configs: %w", err)
	}

	langs := make(map[string]*LangConfig, len(raw))
	for key, doc := range raw {
		var cfg LangConfig
		if err := json.Unmarshal(doc, &cfg); err != nil {
			// One malformed row must not take every language down. It is
			// dropped, which surfaces as "unsupported language" for that key
			// alone — the same failure mode the file loader had for an
			// unparseable YAML file.
			slog.Error("compiler: skipping malformed language config", "key", key, "error", err)
			continue
		}
		if cfg.Key == "" {
			cfg.Key = key
		}
		langs[cfg.Key] = &cfg
	}

	l.langs = langs
	l.loadedAt = l.now()
	l.loaded = true
	return langs, nil
}

// Get returns one language by key.
func (l *Loader) Get(ctx context.Context, key string) (*LangConfig, bool) {
	langs, err := l.All(ctx)
	if err != nil {
		return nil, false
	}
	cfg, ok := langs[key]
	return cfg, ok
}
