// Package lang exposes the judge language configs that ship with the repo.
//
// The YAML files in this directory are the only definition of a language — the
// judge loads them at start-up and the admin panel edits them in place. Editing
// in place means an operator can also break one, and "Reset to Default" needs
// the original bytes to restore.
//
// Embedding is what makes that possible without a second copy: a hardcoded
// default table drifts the moment a file here changes (the previous one still
// advertised `cpp-gcc`, a key no config has used for months). Embedding reads
// the same files that are written to disk, so the two cannot disagree.
//
// The embed directive only matches in this directory, which is why the
// package lives here rather than next to its callers.
package lang

import (
	"embed"
	"path"
	"sort"
)

//go:embed *.yaml
var files embed.FS

// Default returns the shipped bytes for a language key (the file name without
// its extension). The second result is false when no config ships under that
// key, which means the language was created by an operator and has no default
// to restore.
func Default(key string) ([]byte, bool) {
	data, err := files.ReadFile(key + ".yaml")
	if err != nil {
		return nil, false
	}
	return data, true
}

// Keys lists the shipped language keys, sorted.
func Keys() []string {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		keys = append(keys, path.Base(e.Name()[:len(e.Name())-len(path.Ext(e.Name()))]))
	}
	sort.Strings(keys)
	return keys
}
