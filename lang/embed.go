// Package lang exposes the judge language configs that SHIP with the repo.
//
// These files are no longer the live definition of a language: the registry is
// the `languages` table, and the admin panel edits rows. They are the shipped
// defaults — what a fresh install seeds from (DefaultsJSON) and what "Reset to
// Default" restores (Default). Editing in place meant an operator could break a
// language, and reset needs the original bytes to put it back.
//
// Embedding is what makes that possible without a second copy: a hardcoded
// default table drifts the moment a file here changes (the previous one still
// advertised `cpp-gcc`, a key no config has used for months). Embedding reads
// the same files that are compiled into the binary, so the two cannot disagree.
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
