package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// contestStaffStore is the slice of ContestStore the staff-only per-contest
// handlers need: resolve the round, check whether the caller may act on it, and
// read its problems. Narrower than the whole store so the permission rules can be
// tested without standing up every contest method. (Distinct from
// contest_access.go's contestAccess, which is the read-visibility seam.)
type contestStaffStore interface {
	GetByID(ctx context.Context, id string) (*model.Contest, error)
	HasAccess(ctx context.Context, contestID, userID string, levels ...string) bool
	GetProblems(ctx context.Context, contestID string) ([]model.ContestProblem, error)
}

type ClarificationHandler struct {
	clarificationStore store.ClarificationStore
	contestStore       contestStaffStore
}

func NewClarificationHandler(cs store.ClarificationStore, cts contestStaffStore) *ClarificationHandler {
	return &ClarificationHandler{
		clarificationStore: cs,
		contestStore:       cts,
	}
}

func (h *ClarificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestSlug := chi.URLParam(r, "contestId")

	contest, err := h.contestStore.GetByID(r.Context(), contestSlug)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	var req struct {
		ProblemID *string `json:"problem_id,omitempty"`
		Question  string  `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Question == "" {
		http.Error(w, "question required", http.StatusBadRequest)
		return
	}

	c := &model.Clarification{
		ContestID: contest.ID,
		UserID:    claims.UserID,
		ProblemID: req.ProblemID,
		Question:  req.Question,
	}

	if err := h.clarificationStore.Create(r.Context(), c); err != nil {
		http.Error(w, "failed to create clarification", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, c)
}

func (h *ClarificationHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	contestSlug := chi.URLParam(r, "contestId")

	contest, err := h.contestStore.GetByID(r.Context(), contestSlug)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	var userID *string
	isJudge := false
	if claims != nil {
		isJudge = claims.Role == "admin" || h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge")
		if !isJudge {
			uid := claims.UserID
			userID = &uid
		}
	}

	// Filters are optional: an unfiltered list is what the tab has always
	// fetched. They are what makes a round's clarification queue workable — the
	// jury needs "unanswered, about problem C" rather than one long list.
	var problemID *string
	if v := strings.TrimSpace(r.URL.Query().Get("problem_id")); v != "" {
		problemID = &v
	}
	var answered *bool
	if v := strings.TrimSpace(r.URL.Query().Get("answered")); v != "" {
		switch v {
		case "true", "1":
			t := true
			answered = &t
		case "false", "0":
			f := false
			answered = &f
		}
	}

	clarifications, err := h.clarificationStore.ListByContest(r.Context(), contest.ID, userID, problemID, answered)
	if err != nil {
		http.Error(w, "failed to list clarifications: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":     clarifications,
		"is_judge": isJudge,
	})
}

// AnswerMany applies one answer to several clarifications at once, so a jury can
// answer a run of questions with the same ruling instead of one at a time.
func (h *ClarificationHandler) AnswerMany(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contest, err := resolveContest(r.Context(), chi.URLParam(r, "contestId"), h.contestStore)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		IDs      []string `json:"ids"`
		Answer   string   `json:"answer"`
		IsPublic bool     `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Answer == "" {
		http.Error(w, "answer required", http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		http.Error(w, "no clarifications selected", http.StatusBadRequest)
		return
	}

	// Only the ids that belong to this contest. The caller is authorized for the
	// contest, so without this a judge of contest A could answer rows of contest B
	// by guessing their ids.
	scoped := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		c, err := h.clarificationStore.GetByID(r.Context(), id)
		if err != nil || c == nil {
			continue
		}
		if c.ContestID == contest.ID {
			scoped = append(scoped, id)
		}
	}
	if len(scoped) == 0 {
		http.Error(w, "none of these clarifications belong to this contest", http.StatusBadRequest)
		return
	}

	if err := h.clarificationStore.AnswerMany(r.Context(), scoped, req.Answer, claims.UserID, req.IsPublic); err != nil {
		http.Error(w, "failed to answer clarifications", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"answered": len(scoped)})
}

func (h *ClarificationHandler) Answer(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contest, err := resolveContest(r.Context(), chi.URLParam(r, "contestId"), h.contestStore)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	clarificationID := chi.URLParam(r, "id")

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Answer   string `json:"answer"`
		IsPublic bool   `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Answer == "" {
		http.Error(w, "answer required", http.StatusBadRequest)
		return
	}

	if err := h.clarificationStore.Answer(r.Context(), clarificationID, req.Answer, claims.UserID, req.IsPublic); err != nil {
		http.Error(w, "failed to answer clarification", http.StatusInternalServerError)
		return
	}

	// The row as it now stands, so the caller can replace it in place instead of
	// refetching the whole list (which is what the queue does after every reply).
	c, err := h.clarificationStore.GetByID(r.Context(), clarificationID)
	if err != nil || c == nil {
		respondJSON(w, http.StatusOK, map[string]string{"status": "answered"})
		return
	}
	respondJSON(w, http.StatusOK, c)
}

type ContestNoticeHandler struct {
	noticeStore  store.ContestNoticeStore
	contestStore contestReadAccess
}

func NewContestNoticeHandler(ns store.ContestNoticeStore, cts contestReadAccess) *ContestNoticeHandler {
	return &ContestNoticeHandler{
		noticeStore:  ns,
		contestStore: cts,
	}
}

func (h *ContestNoticeHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestSlug := chi.URLParam(r, "contestId")

	contest, err := h.contestStore.GetByID(r.Context(), contestSlug)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content required", http.StatusBadRequest)
		return
	}

	notice := &model.ContestNotice{
		ContestID: contest.ID,
		Content:   req.Content,
		CreatedBy: claims.UserID,
	}

	if err := h.noticeStore.Create(r.Context(), notice); err != nil {
		http.Error(w, "failed to create notice", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, notice)
}

func (h *ContestNoticeHandler) List(w http.ResponseWriter, r *http.Request) {
	contestSlug := chi.URLParam(r, "contestId")

	contest, err := h.contestStore.GetByID(r.Context(), contestSlug)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	notices, err := h.noticeStore.ListByContest(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "failed to list notices", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": notices,
	})
}

func (h *ContestNoticeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestSlug := chi.URLParam(r, "contestId")
	noticeID := chi.URLParam(r, "id")

	contest, err := h.contestStore.GetByID(r.Context(), contestSlug)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.noticeStore.Delete(r.Context(), noticeID); err != nil {
		http.Error(w, "failed to delete notice", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
