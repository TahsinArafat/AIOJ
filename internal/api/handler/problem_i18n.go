package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// ProblemI18nHandler exposes per-problem translations.
//
// Read access is public (a solver viewing a problem in Bengali must be able to
// fetch it); writes mirror the existing admin-language pattern and require an
// authenticated request, with ownership enforced by the route's middleware.
type ProblemI18nHandler struct {
	i18nStore    store.ProblemI18nStore
	problemStore store.ProblemStore
}

func NewProblemI18nHandler(i18nStore store.ProblemI18nStore, problemStore store.ProblemStore) *ProblemI18nHandler {
	return &ProblemI18nHandler{i18nStore: i18nStore, problemStore: problemStore}
}

// BCP-47-ish language tag: 2-3 letters, optionally with a region/script subtag.
// Deliberately narrow so a language code can't be arbitrary text -- these values
// become part of a primary key and are echoed into the frontend.
var languageTagRe = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)

func validLanguage(lang string) bool {
	return languageTagRe.MatchString(lang)
}

// problem resolves {id} and reports whether the caller may read it. It writes
// the error response itself and returns nil on failure. Translations carry the
// statement text, so a draft's translations are as private as the draft.
func (h *ProblemI18nHandler) readableProblem(w http.ResponseWriter, r *http.Request) *model.Problem {
	p, err := h.problemStore.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return nil
	}
	if p == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return nil
	}
	if !canReadProblem(r.Context(), h.problemStore, p, middleware.GetUserClaims(r)) {
		http.Error(w, "problem not found", http.StatusNotFound)
		return nil
	}
	return p
}

// authorProblem is readableProblem plus the right to change the statement.
func (h *ProblemI18nHandler) authorProblem(w http.ResponseWriter, r *http.Request) *model.Problem {
	p := h.readableProblem(w, r)
	if p == nil {
		return nil
	}
	claims := middleware.GetUserClaims(r)
	if claims == nil || (claims.Role != "admin" && p.CreatedBy != claims.UserID &&
		!h.problemStore.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil
	}
	return p
}

// List returns every translation for a problem.
func (h *ProblemI18nHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.readableProblem(w, r) == nil {
		return
	}
	problemID := chi.URLParam(r, "id")
	items, err := h.i18nStore.ListForProblem(r.Context(), problemID)
	if err != nil {
		slog.Error("list problem i18n failed", "problem_id", problemID, "error", err)
		http.Error(w, "failed to list translations", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// Upsert creates or replaces one language's translation.
func (h *ProblemI18nHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "id")
	lang := chi.URLParam(r, "lang")

	if !validLanguage(lang) {
		http.Error(w, "invalid language tag", http.StatusBadRequest)
		return
	}

	// Confirm the problem exists, is readable, and that this caller may change
	// its statement, so we return 404/403 rather than a foreign-key violation
	// (which would leak a 500 for what is really a client error).
	if h.authorProblem(w, r) == nil {
		return
	}

	var body struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	title := body.Title
	description := body.Description
	if title == nil && description == nil {
		http.Error(w, "title or description required", http.StatusBadRequest)
		return
	}
	if title != nil && strings.TrimSpace(*title) == "" {
		http.Error(w, "title must not be blank", http.StatusBadRequest)
		return
	}

	t := &model.ProblemI18n{
		ProblemID:   problemID,
		Language:    lang,
		Title:       title,
		Description: description,
	}
	if err := h.i18nStore.Upsert(r.Context(), t); err != nil {
		slog.Error("upsert problem i18n failed", "problem_id", problemID, "lang", lang, "error", err)
		http.Error(w, "failed to save translation", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// Delete removes a translation, reverting that language to the default text.
func (h *ProblemI18nHandler) Delete(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "id")
	lang := chi.URLParam(r, "lang")

	if !validLanguage(lang) {
		http.Error(w, "invalid language tag", http.StatusBadRequest)
		return
	}
	if h.authorProblem(w, r) == nil {
		return
	}
	if err := h.i18nStore.Delete(r.Context(), problemID, lang); err != nil {
		slog.Error("delete problem i18n failed", "problem_id", problemID, "lang", lang, "error", err)
		http.Error(w, "failed to delete translation", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
