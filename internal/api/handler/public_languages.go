package handler

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
)

// PublicLanguageHandler serves the judge language list to the SPA.
//
// The IDE, the problem page and the contest problem page each carried their own
// copy of the language list in TypeScript, so they advertised whatever that copy
// said rather than what the judge actually runs. Saving a language in
// admin#languages — or adding one — changed nothing on those pages: they kept
// offering the hardcoded set, and offering a key the judge had no config for
// produced a submission that failed at judge time.
//
// This handler reads the same lang/*.yaml directory the judge reads, on every
// request, so an admin edit is visible on the next page load. The files are
// small (a dozen, a few hundred bytes each) and this endpoint is hit once per
// page load, not per keystroke, so caching would buy nothing and only add a way
// for the UI to disagree with the judge.
type PublicLanguageHandler struct {
	langDir string
}

func NewPublicLanguageHandler(langDir string) *PublicLanguageHandler {
	return &PublicLanguageHandler{langDir: langDir}
}

// PublicLanguage is the wire shape for one selectable language. It carries only
// what a submit form needs: the key it sends back, the name it shows, and the
// file extensions the editor can use to pick a syntax mode.
//
// Compile and runtime commands are deliberately absent — they name absolute
// paths inside the judge image, and publishing them hands an attacker a map of
// the sandbox for no UI benefit.
type PublicLanguage struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
}

func (h *PublicLanguageHandler) List(w http.ResponseWriter, r *http.Request) {
	langs, err := compiler.LoadLanguages(h.langDir)
	if err != nil {
		// A missing lang directory is an install fault, not a client error.
		// Answering with an empty list here would render an empty language
		// dropdown and look like "this judge accepts no languages".
		http.Error(w, "failed to load languages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sorted by key so the dropdown order is stable across requests. A map
	// iteration order would shuffle the list on every reload.
	keys := make([]string, 0, len(langs))
	for key := range langs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]PublicLanguage, 0, len(keys))
	for _, key := range keys {
		cfg := langs[key]
		exts := cfg.Extensions
		if exts == nil {
			// Marshal as [] not null: the SPA maps over this.
			exts = []string{}
		}
		result = append(result, PublicLanguage{Key: cfg.Key, Name: cfg.Name, Extensions: exts})
	}

	w.Header().Set("Content-Type", "application/json")
	// Short and public: the list changes only when an admin saves a language,
	// and a stale minute is harmless. This is not an authorization boundary.
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(map[string]any{"data": result})
}
