package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/plagiarism"
	"github.com/tahsinarafat/aioj/internal/store"
)

// maxConcurrentPlagiarismChecks bounds how many O(n²) similarity scans may run
// at once. Each scan is CPU-bound and compares every accepted submission of
// every problem in a contest, so unbounded runs could stall the whole process.
const maxConcurrentPlagiarismChecks = 2

// plagiarismCheckTimeout caps a single scan. The goroutine used to run with
// context.Background() and no deadline.
const plagiarismCheckTimeout = 10 * time.Minute

type PlagiarismHandler struct {
	service      *plagiarism.Service
	store        store.PlagiarismStore
	contestStore store.ContestStore
	sem          chan struct{}
}

func NewPlagiarismHandler(s *plagiarism.Service, plStore store.PlagiarismStore, contestStore store.ContestStore) *PlagiarismHandler {
	return &PlagiarismHandler{
		service:      s,
		store:        plStore,
		contestStore: contestStore,
		sem:          make(chan struct{}, maxConcurrentPlagiarismChecks),
	}
}

// requireContestStaff authorizes the caller as a platform admin, the contest
// creator, or a contest manager/judge, and returns the resolved contest.
//
// Plagiarism reports contain other contestants' submission pairs, so they are
// staff-only. These routes previously checked authentication alone: any logged-in
// user could start a scan on any contest, read the resulting report, and
// overwrite pair verdicts.
func (h *PlagiarismHandler) requireContestStaff(w http.ResponseWriter, r *http.Request, contestID string) (*model.Contest, bool) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	if contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return nil, false
	}
	if claims.Role == "admin" || contest.CreatedBy == claims.UserID ||
		h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge") {
		return contest, true
	}
	http.Error(w, "forbidden", http.StatusForbidden)
	return nil, false
}

// requireReportStaff resolves a report ID to its contest and authorizes the
// caller against that contest.
func (h *PlagiarismHandler) requireReportStaff(w http.ResponseWriter, r *http.Request, reportID string) (*model.PlagiarismReport, bool) {
	report, err := h.store.GetReportByID(r.Context(), reportID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	if report == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil, false
	}
	if _, ok := h.requireContestStaff(w, r, report.ContestID); !ok {
		return nil, false
	}
	return report, true
}

func (h *PlagiarismHandler) RunCheck(w http.ResponseWriter, r *http.Request) {
	var req model.PlagiarismCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	contest, ok := h.requireContestStaff(w, r, req.ContestID)
	if !ok {
		return
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.70
	}
	if threshold > 1 {
		http.Error(w, "threshold must be between 0 and 1", http.StatusBadRequest)
		return
	}

	// Refuse rather than queue: a scan already running will produce a report
	// the caller can read, and piling up CPU-bound goroutines helps nobody.
	select {
	case h.sem <- struct{}{}:
	default:
		http.Error(w, "a plagiarism check is already running; retry shortly",
			http.StatusTooManyRequests)
		return
	}

	claims := middleware.GetUserClaims(r)
	report := &model.PlagiarismReport{
		ContestID: contest.ID,
		Threshold: threshold,
		CreatedBy: claims.UserID,
	}
	if err := h.store.CreateReport(r.Context(), report); err != nil {
		<-h.sem
		http.Error(w, "failed to create report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		defer func() { <-h.sem }()
		ctx, cancel := context.WithTimeout(context.Background(), plagiarismCheckTimeout)
		defer cancel()
		h.service.RunCheck(ctx, report.ID, contest.ID, threshold)
	}()

	respondJSON(w, http.StatusAccepted, report)
}

func (h *PlagiarismHandler) GetReportByContest(w http.ResponseWriter, r *http.Request) {
	contest, ok := h.requireContestStaff(w, r, chi.URLParam(r, "contestId"))
	if !ok {
		return
	}

	report, err := h.store.GetReportByContest(r.Context(), contest.ID)
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
	report, ok := h.requireReportStaff(w, r, chi.URLParam(r, "reportId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, report)
}

func (h *PlagiarismHandler) ListPairs(w http.ResponseWriter, r *http.Request) {
	report, ok := h.requireReportStaff(w, r, chi.URLParam(r, "reportId"))
	if !ok {
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	pairs, total, err := h.store.ListPairsByReport(r.Context(), report.ID, offset, limit)
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
	switch req.Status {
	case model.PlagiarismPairPending, model.PlagiarismPairFlagged, model.PlagiarismPairIgnored:
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	// A pair belongs to a report; authorize against that report's contest.
	pair, err := h.store.GetPairByID(r.Context(), pairID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if pair == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if _, ok := h.requireReportStaff(w, r, pair.ReportID); !ok {
		return
	}

	if err := h.store.UpdatePairStatus(r.Context(), pairID, req.Status); err != nil {
		http.Error(w, "failed to update pair status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
