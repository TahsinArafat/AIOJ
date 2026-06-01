package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type ClarificationHandler struct {
	clarificationStore store.ClarificationStore
	contestStore       store.ContestStore
}

func NewClarificationHandler(cs store.ClarificationStore, cts store.ContestStore) *ClarificationHandler {
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
	if claims != nil {
		isJudge := claims.Role == "admin" || h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge")
		if !isJudge {
			uid := claims.UserID
			userID = &uid
		}
	}

	clarifications, err := h.clarificationStore.ListByContest(r.Context(), contest.ID, userID)
	if err != nil {
		http.Error(w, "failed to list clarifications: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": clarifications,
	})
}

func (h *ClarificationHandler) Answer(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "contestId")
	clarificationID := chi.URLParam(r, "id")

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager", "judge") {
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

	respondJSON(w, http.StatusOK, map[string]string{"status": "answered"})
}

type ContestNoticeHandler struct {
	noticeStore  store.ContestNoticeStore
	contestStore store.ContestStore
}

func NewContestNoticeHandler(ns store.ContestNoticeStore, cts store.ContestStore) *ContestNoticeHandler {
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
