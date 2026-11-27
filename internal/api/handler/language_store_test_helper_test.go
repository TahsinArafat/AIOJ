package handler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/model"
)

// memLanguageStore is an in-memory store.LanguageStore for handler tests.
//
// The handlers talk to the registry now, not to files, so a test that wants to
// see "what got saved" reads this instead of a lang/*.yaml file. It can still be
// seeded from a directory (fromDir) so the existing fixtures — which are the
// real LangConfig YAML — keep exercising the real decoder.
type memLanguageStore struct {
	rows    map[string]json.RawMessage
	deleted map[string]bool
}

func newMemLanguageStore() *memLanguageStore {
	return &memLanguageStore{rows: map[string]json.RawMessage{}, deleted: map[string]bool{}}
}

// fromDir seeds the store from a lang/ directory of YAML fixtures.
func (s *memLanguageStore) fromDir(t *testing.T, dir string) *memLanguageStore {
	t.Helper()
	langs, err := compiler.LoadLanguages(dir)
	if err != nil {
		t.Fatalf("load fixture langs from %s: %v", dir, err)
	}
	for key, cfg := range langs {
		doc, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("encode fixture %s: %v", key, err)
		}
		s.rows[key] = doc
	}
	return s
}

func (s *memLanguageStore) List(context.Context) ([]model.Language, error) {
	out := make([]model.Language, 0, len(s.rows))
	for key, doc := range s.rows {
		if s.deleted[key] {
			continue
		}
		out = append(out, model.Language{Key: key, Config: doc, UpdatedAt: time.Now()})
	}
	return out, nil
}

func (s *memLanguageStore) GetByKey(_ context.Context, key string) (*model.Language, error) {
	doc, ok := s.rows[key]
	if !ok || s.deleted[key] {
		return nil, nil
	}
	return &model.Language{Key: key, Config: doc, UpdatedAt: time.Now()}, nil
}

func (s *memLanguageStore) Upsert(_ context.Context, key string, config json.RawMessage, _ string) error {
	s.rows[key] = config
	delete(s.deleted, key)
	return nil
}

func (s *memLanguageStore) Delete(_ context.Context, key string) error {
	s.deleted[key] = true
	return nil
}

func (s *memLanguageStore) Seed(_ context.Context, langs map[string]json.RawMessage) (int, error) {
	created := 0
	for key, doc := range langs {
		if _, ok := s.rows[key]; ok {
			continue
		}
		s.rows[key] = doc
		created++
	}
	return created, nil
}

// ListLanguageConfigs lets this store back a compiler.Loader.
func (s *memLanguageStore) ListLanguageConfigs(ctx context.Context) (map[string]json.RawMessage, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(items))
	for _, l := range items {
		out[l.Key] = l.Config
	}
	return out, nil
}

// lang decodes one saved config, failing the test if it is absent.
func (s *memLanguageStore) lang(t *testing.T, key string) compiler.LangConfig {
	t.Helper()
	doc, ok := s.rows[key]
	if !ok || s.deleted[key] {
		t.Fatalf("language %q was not saved", key)
	}
	var cfg compiler.LangConfig
	if err := json.Unmarshal(doc, &cfg); err != nil {
		t.Fatalf("saved config %q is not valid JSON: %v\n%s", key, err, doc)
	}
	return cfg
}

// newLangHandler builds an admin handler over an in-memory registry.
func newLangHandler(st *memLanguageStore) *AdminLanguageHandler {
	return NewAdminLanguageHandler(st, compiler.NewLoader(st), nil)
}

// newFileLoader builds a loader over a lang/ directory, with caching disabled so
// every read re-reads. Tests that assert "the next request sees the edit" depend
// on that; production keeps the TTL.
func newFileLoader(dir string) *compiler.Loader {
	l := compiler.NewLoader(compiler.NewFileSource(dir))
	l.SetTTL(0)
	return l
}
