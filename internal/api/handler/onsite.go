package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type OnsiteHandler struct {
	balloonStore *postgres.BalloonStore
	printStore   *postgres.PrintStore
	contestStore *postgres.ContestStore
}

func NewOnsiteHandler(bs *postgres.BalloonStore, ps *postgres.PrintStore, cs *postgres.ContestStore) *OnsiteHandler {
	return &OnsiteHandler{
		balloonStore: bs,
		printStore:   ps,
		contestStore: cs,
	}
}

func (h *OnsiteHandler) ListBalloons(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify manager or admin access
	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager", "tester") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	list, err := h.balloonStore.ListByContest(r.Context(), contestID)
	if err != nil {
		http.Error(w, "failed to list balloons: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}

func (h *OnsiteHandler) DispatchBalloon(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	balloonID := chi.URLParam(r, "balloonId")
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager", "tester") {
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
	contestID := chi.URLParam(r, "id")
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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

	err := h.printStore.Create(r.Context(), contestID, claims.UserID, req.Filename, req.Content)
	if err != nil {
		http.Error(w, "failed to create print request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *OnsiteHandler) ListPrints(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager", "tester") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	list, err := h.printStore.ListByContest(r.Context(), contestID)
	if err != nil {
		http.Error(w, "failed to list prints: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": list})
}

func (h *OnsiteHandler) UpdatePrintStatus(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	printID := chi.URLParam(r, "printId")
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager", "tester") {
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
