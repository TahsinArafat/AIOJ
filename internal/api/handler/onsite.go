package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type contestLookup interface {
	GetByID(context.Context, string) (*model.Contest, error)
	HasAccess(context.Context, string, string, ...string) bool
}

type OnsiteHandler struct {
	balloonStore store.BalloonStore
	printStore   store.PrintStore
	contestStore contestLookup
}

func NewOnsiteHandler(bs store.BalloonStore, ps store.PrintStore, cs contestLookup) *OnsiteHandler {
	return &OnsiteHandler{
		balloonStore: bs,
		printStore:   ps,
		contestStore: cs,
	}
}

func (h *OnsiteHandler) resolveContest(w http.ResponseWriter, r *http.Request) (*model.Contest, bool) {
	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.contestStore)
	if err != nil {
		respondContestLookupError(w, err)
		return nil, false
	}
	return contest, true
}

func (h *OnsiteHandler) ListBalloons(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	contest, ok := h.resolveContest(w, r)
	if !ok {
		return
	}

	// Verify manager or admin access against the canonical contest UUID.
	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	list, err := h.balloonStore.ListByContest(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "failed to list balloons: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}

func (h *OnsiteHandler) DispatchBalloon(w http.ResponseWriter, r *http.Request) {
	balloonID := chi.URLParam(r, "balloonId")
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	contest, ok := h.resolveContest(w, r)
	if !ok {
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	err := h.balloonStore.Dispatch(r.Context(), balloonID)
	if err != nil {
		http.Error(w, "failed to dispatch balloon: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *OnsiteHandler) RequestPrint(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	contest, ok := h.resolveContest(w, r)
	if !ok {
		return
	}

	var req struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Filename == "" {
		req.Filename = "solution.cpp"
	}
	if req.Content == "" {
		http.Error(w, "print content cannot be empty", http.StatusBadRequest)
		return
	}

	err := h.printStore.Create(r.Context(), contest.ID, claims.UserID, req.Filename, req.Content)
	if err != nil {
		http.Error(w, "failed to create print request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *OnsiteHandler) ListPrints(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	contest, ok := h.resolveContest(w, r)
	if !ok {
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	list, err := h.printStore.ListByContest(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "failed to list prints: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}

func (h *OnsiteHandler) UpdatePrintStatus(w http.ResponseWriter, r *http.Request) {
	printID := chi.URLParam(r, "printId")
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	contest, ok := h.resolveContest(w, r)
	if !ok {
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Status != "printed" && req.Status != "cancelled" && req.Status != "pending" {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	err := h.printStore.UpdateStatus(r.Context(), printID, req.Status)
	if err != nil {
		http.Error(w, "failed to update print status: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
