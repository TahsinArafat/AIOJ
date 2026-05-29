package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/plagiarism"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type PlagiarismHandler struct {
	service *plagiarism.Service
	store   *postgres.PlagiarismStore
}

func NewPlagiarismHandler(s *plagiarism.Service, store *postgres.PlagiarismStore) *PlagiarismHandler {
	return &PlagiarismHandler{service: s, store: store}
}

func (h *PlagiarismHandler) RunCheck(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.PlagiarismCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.70
	}

	report := &model.PlagiarismReport{
		ContestID: req.ContestID,
		Threshold: threshold,
		CreatedBy: claims.UserID,
	}

	if err := h.store.CreateReport(r.Context(), report); err != nil {
		http.Error(w, "failed to create report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	go h.service.RunCheck(r.Context(), report.ID, req.ContestID, threshold)

	respondJSON(w, http.StatusAccepted, report)
}

func (h *PlagiarismHandler) GetReportByContest(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	report, err := h.store.GetReportByContest(r.Context(), contestID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if report == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, report)
}

func (h *PlagiarismHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportId")
	report, err := h.store.GetReportByID(r.Context(), reportID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if report == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, report)
}

func (h *PlagiarismHandler) ListPairs(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportId")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	pairs, total, err := h.store.ListPairsByReport(r.Context(), reportID, offset, limit)
	if err != nil {
		http.Error(w, "failed to list pairs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": pairs, "total": total})
}

func (h *PlagiarismHandler) UpdatePairStatus(w http.ResponseWriter, r *http.Request) {
	pairID := chi.URLParam(r, "pairId")
	var req model.PlagiarismPairUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdatePairStatus(r.Context(), pairID, req.Status); err != nil {
		http.Error(w, "failed to update pair status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
