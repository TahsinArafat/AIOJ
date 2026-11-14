package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/vjudge"
)

// remotePollTimeout bounds a detached remote-OJ poll. The poll is fire-and-
// forget from the caller's point of view, so it needs its own deadline.
const remotePollTimeout = 2 * time.Minute

// adminSubmissionStore is what the admin submissions console touches: the
// rejudge and failure-triage operations, plus the remote hand-off fields. It
// deliberately excludes Create, UpdateResult and the listing methods the
// ordinary submission handler owns.
type adminSubmissionStore interface {
	store.SubmissionRemoteJudge
	GetByID(ctx context.Context, id string) (*model.Submission, error)
	ListRecentFailures(ctx context.Context, limit int) ([]model.Submission, error)
	ListForBulkRejudge(ctx context.Context, contestID, problemID string) ([]string, error)
	ListIDsForProblemRejudge(ctx context.Context, problemID string, filter model.SubmissionFilter) ([]string, error)
	ResetForRejudge(ctx context.Context, ids []string) error
	RecomputeProblemCounts(ctx context.Context, problemID string) error
	UpdateStatus(ctx context.Context, id string, status model.SubmissionStatus)
	UpdateResult(ctx context.Context, id string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) error
}

type AdminSubmissionHandler struct {
	subStore  adminSubmissionStore
	probStore store.ProblemReader
	queue     queue.JudgeQueue
	vjudgeSvc *vjudge.Service
}

func NewAdminSubmissionHandler(subStore adminSubmissionStore, probStore store.ProblemReader, vjSvc *vjudge.Service) *AdminSubmissionHandler {
	return &AdminSubmissionHandler{subStore: subStore, probStore: probStore, vjudgeSvc: vjSvc}
}

// WithQueue attaches the judge queue so admin rejudge endpoints can enqueue
// local-problem submissions.  Call this after NewAdminSubmissionHandler.
func (h *AdminSubmissionHandler) WithQueue(q queue.JudgeQueue) *AdminSubmissionHandler {
	h.queue = q
	return h
}

func (h *AdminSubmissionHandler) ListPendingRemote(w http.ResponseWriter, r *http.Request) {
	subs, err := h.subStore.GetPendingRemoteSubmissions(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": len(subs)})
}

// ListFailed returns the newest System Error submissions with the judge's own
// failure text. This is the "why did it crash" view: a contestant only ever
// sees "System Error", so without this the cause lived in the worker's stdout.
//
// GET /api/admin/submissions/failed?limit=50
func (h *AdminSubmissionHandler) ListFailed(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	subs, err := h.subStore.ListRecentFailures(r.Context(), limit)
	if err != nil {
		slog.Error("list judge failures", "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list failures"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": len(subs)})
}

// Rejudge handles both local and remote OJ submissions.
//
// For remote OJ problems it resets the remote ID/bot fields so the vjudge
// submit worker picks the row back up.
//
// For local problems it resets status to 'pending' and re-enqueues at
// PriorityNormal (admin/manual rejudge should not jump live contest traffic).
func (h *AdminSubmissionHandler) Rejudge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "submission not found"})
		return
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err != nil || prob == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "problem not found"})
		return
	}

	isRemote := model.IsRemotePlatform(prob.Source)

	if isRemote {
		// Remote OJ: clear remote tracking so the submit worker resubmits.
		h.subStore.UpdateStatus(r.Context(), id, model.StatusPending)
		h.subStore.UpdateRemoteID(r.Context(), id, "", "")
		h.subStore.UpdateBotID(r.Context(), id, "", "")
		respondJSON(w, http.StatusOK, map[string]string{"status": "rejudging"})
		return
	}

	// Local problem: reset and re-enqueue.
	if h.queue == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "judge queue not available"})
		return
	}
	h.subStore.UpdateStatus(r.Context(), id, model.StatusPending)
	if err := h.queue.Enqueue(r.Context(), id, PriorityNormal); err != nil {
		slog.Error("admin rejudge: enqueue failed", "id", id, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enqueue"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "rejudging"})
}

func (h *AdminSubmissionHandler) ForceRefresh(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "submission not found"})
		return
	}
	if sub.RemoteID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "no remote submission ID"})
		return
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err != nil || prob == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "problem not found"})
		return
	}

	if h.vjudgeSvc != nil {
		go func() {
			// Detached from the request: the handler returns immediately and its
			// context would cancel the poll straight away.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), remotePollTimeout)
			defer cancel()
			h.vjudgeSvc.ForcePoll(ctx, prob.Source, sub.RemoteID, sub.ID)
		}()
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "refreshing"})
}

// RejudgeBulk re-enqueues all judged local submissions for a contest (or
// optionally a specific problem within that contest) at PriorityBulk.
// This is the right tool after fixing test data mid-contest: it won't delay
// live contest submissions because PriorityBulk is the lowest priority tier.
//
// POST /api/admin/submissions/rejudge-bulk
//
//	{ "contest_id": "...", "problem_id": "..." (optional) }
func (h *AdminSubmissionHandler) RejudgeBulk(w http.ResponseWriter, r *http.Request) {
	if h.queue == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "judge queue not available"})
		return
	}

	var req struct {
		ContestID string `json:"contest_id"`
		ProblemID string `json:"problem_id,omitempty"` // optional — all problems if empty
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ContestID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "contest_id is required"})
		return
	}

	ids, err := h.subStore.ListForBulkRejudge(r.Context(), req.ContestID, req.ProblemID)
	if err != nil {
		slog.Error("bulk rejudge: list failed", "contest", req.ContestID, "problem", req.ProblemID, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list submissions"})
		return
	}
	if len(ids) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": 0})
		return
	}

	// Reset all matching rows to 'pending' in the DB first (single UPDATE).
	if err := h.subStore.ResetForRejudge(r.Context(), ids); err != nil {
		slog.Error("bulk rejudge: reset failed", "contest", req.ContestID, "count", len(ids), "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset submissions"})
		return
	}

	// Enqueue at PriorityBulk — lower than live contest (PriorityContest=1)
	// and normal practice (PriorityNormal=2), so bulk work drains last.
	enqueued := 0
	for _, id := range ids {
		if err := h.queue.Enqueue(r.Context(), id, PriorityBulk); err != nil {
			slog.Error("bulk rejudge: enqueue failed", "id", id, "error", err)
			continue
		}
		enqueued++
	}
	slog.Info("bulk rejudge enqueued", "contest", req.ContestID, "problem", req.ProblemID,
		"total", len(ids), "enqueued", enqueued)
	respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": enqueued, "total": len(ids)})
}
