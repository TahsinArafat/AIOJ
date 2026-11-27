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
// This handler reads the same registry the judge reads, through the shared
// loader, so an admin edit is visible on the next page load. It is the endpoint
// that always was: it re-read on every request while the judge cached forever,
// which is how a language could appear in the submit dropdown and still be
// rejected as "unsupported" at judge time.
type PublicLanguageHandler struct {
	loader *compiler.Loader
}

func NewPublicLanguageHandler(loader *compiler.Loader) *PublicLanguageHandler {
	return &PublicLanguageHandler{loader: loader}
}

// PublicLanguage is the wire shape for one selectable language. It carries only
// what a submit form needs: the key it sends back, the name it shows, the file
// extensions the editor can use to pick a syntax mode, and the resource limits
// the judge will actually apply.
//
// Compile and runtime commands are deliberately absent — they name absolute
// paths inside the judge image, and publishing them hands an attacker a map of
// the sandbox for no UI benefit. The limits are the opposite case: the problem
// page has to show the limit a submission will really be judged under, and that
// is the problem's own limit scaled by the language's multipliers. Without them
// the page could only show the raw problem row, which is not what the sandbox
// enforces.
type PublicLanguage struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
	// Multipliers, not absolute values: the base is per-problem.
	TimeLimitMultiplier   float64 `json:"time_limit_multiplier"`
	MemoryLimitMultiplier float64 `json:"memory_limit_multiplier"`
	// StackLimitKB is the call-stack cap the judge applies, 0 for the sandbox
	// default.
	StackLimitKB int `json:"stack_limit_kb"`
}

func (h *PublicLanguageHandler) List(w http.ResponseWriter, r *http.Request) {
	langs, err := h.loader.All(r.Context())
	if err != nil {
		// An unreachable registry is an install fault, not a client error.
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
		// A config saved before a field existed leaves it at 0, and a 0×
		// multiplier would render as "0 s" and tell the user nothing. 1.0 is
		// what the admin handlers store for an unset multiplier anyway.
		timeMul, memMul := cfg.TimeLimitMultiplier, cfg.MemoryLimitMultiplier
		if timeMul <= 0 {
			timeMul = 1.0
		}
		if memMul <= 0 {
			memMul = 1.0
		}
		result = append(result, PublicLanguage{
			Key:                   cfg.Key,
			Name:                  cfg.Name,
			Extensions:            exts,
			TimeLimitMultiplier:   timeMul,
			MemoryLimitMultiplier: memMul,
			StackLimitKB:          cfg.StackLimitKB,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	// Short and public: the list changes only when an admin saves a language,
	// and a stale minute is harmless. This is not an authorization boundary.
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(map[string]any{"data": result})
}
