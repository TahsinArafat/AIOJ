package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
)

type SubmissionHandler struct {
	subStore     store.SubmissionStore
	probStore    store.ProblemStore
	contestStore store.ContestStore
	queue        queue.JudgeQueue
	ws           *WSManager
}

func NewSubmissionHandler(s store.SubmissionStore, p store.ProblemStore, cs store.ContestStore, q queue.JudgeQueue, ws *WSManager) *SubmissionHandler {
	return &SubmissionHandler{subStore: s, probStore: p, contestStore: cs, queue: q, ws: ws}
}

func (h *SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" || req.Language == "" || req.SourceCode == "" {
		http.Error(w, "problem_id, language, source_code required", http.StatusBadRequest)
		return
	}
	if len(req.SourceCode) > 256*1024 {
		http.Error(w, "code exceeds 256KB", http.StatusBadRequest)
		return
	}
	prob, _ := h.probStore.GetByID(r.Context(), req.ProblemID)
	if prob == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	sub := &model.Submission{
		ID:         uuid.New().String(),
		ProblemID:  req.ProblemID,
		UserID:     claims.UserID,
		ContestID:  req.ContestID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		CodeSize:   len(req.SourceCode),
		Status:     model.StatusPending,
	}
	if err := h.subStore.Create(r.Context(), sub); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	h.queue.Enqueue(r.Context(), sub.ID)
	respondJSON(w, http.StatusCreated, sub)
}

func (h *SubmissionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	sub, _ := h.subStore.GetByID(r.Context(), chi.URLParam(r, "id"))
	if sub == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	claims := middleware.GetUserClaims(r)
	if claims == nil || (claims.UserID != sub.UserID && claims.Role != "admin") {
		sub.SourceCode = ""
	}
	respondJSON(w, http.StatusOK, sub)
}

func (h *SubmissionHandler) ListByProblem(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, total, _ := h.subStore.ListByProblem(r.Context(), chi.URLParam(r, "slug"), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *SubmissionHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, total, _ := h.subStore.ListByUser(r.Context(), claims.UserID, offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *SubmissionHandler) CreateUpsolving(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.ContestID != "" {
		contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
		if err != nil || contest == nil {
			http.Error(w, "contest not found", http.StatusNotFound)
			return
		}

		if time.Now().Before(contest.EndTime) {
			http.Error(w, "contest hasn't ended yet", http.StatusBadRequest)
			return
		}
	}

	submission := &model.Submission{
		ID:         uuid.New().String(),
		ProblemID:  req.ProblemID,
		UserID:     claims.UserID,
		ContestID:  req.ContestID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		CodeSize:   len(req.SourceCode),
		Status:     model.StatusPending,
	}

	if err := h.subStore.Create(r.Context(), submission); err != nil {
		http.Error(w, "submit failed", http.StatusInternalServerError)
		return
	}

	h.queue.Enqueue(r.Context(), submission.ID)

	respondJSON(w, http.StatusCreated, submission)
}
