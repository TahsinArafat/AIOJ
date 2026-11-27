package compiler

import (
	"context"
	"encoding/json"
)

// FileSource serves language configs from a lang/ directory on disk.
//
// It exists for tests, which assert against the shipped lang/*.yaml without a
// database, and for any caller that genuinely wants the files rather than the
// registry. Production does not use it: the registry is the source of truth, and
// reading files at run time is exactly what let an admin edit be lost to the
// container's writable layer.
type FileSource struct {
	Dir string
}

func NewFileSource(dir string) *FileSource { return &FileSource{Dir: dir} }

func (f *FileSource) ListLanguageConfigs(_ context.Context) (map[string]json.RawMessage, error) {
	langs, err := LoadLanguages(f.Dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(langs))
	for key, cfg := range langs {
		doc, err := json.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		out[key] = doc
	}
	return out, nil
}
