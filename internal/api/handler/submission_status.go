package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

var (
	errSubmissionNotOnProblem = errors.New("submission does not belong to this problem")
	errSubmissionInFlight     = errors.New("submission is already queued for judging")
)

// pageParams reads offset/limit with the same bounds every listing uses.
func pageParams(r *http.Request) (offset, limit int) {
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return offset, limit
}

// canManageProblem reports whether the caller may author or rejudge a problem.
// Admins always can; everybody else needs an owner/co_author permission row.
func (h *SubmissionHandler) canManageProblem(r *http.Request, problemID string) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		return false
	}
	if claims.Role == "admin" {
		return true
	}
	return h.probStore.HasAccess(r.Context(), problemID, claims.UserID, "owner", "co_author")
}

// ListAll serves GET /api/submissions/all — the site-wide status page.
//
// Rows the viewer may not open (hidden problem, or a contest still running) are
// excluded in SQL, and every row carries the viewer's own capabilities so the
// table can decide between a link to the submission and plain text.
func (h *SubmissionHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	q := r.URL.Query()
	offset, limit := pageParams(r)

	filter := model.PublicSubmissionFilter{
		SubmissionFilter: model.SubmissionFilter{
			ProblemID: q.Get("problem_id"),
			UserID:    q.Get("user_id"),
			Username:  q.Get("handle"),
			ContestID: q.Get("contest_id"),
			Language:  q.Get("language"),
			Status:    q.Get("status"),
		},
		ProblemSlug: q.Get("problem"),
	}
	if claims != nil {
		filter.ViewerID = claims.UserID
		filter.ShowAll = claims.Role == "admin"
	}

	subs, total, err := h.subStore.ListPublic(r.Context(), offset, limit, filter)
	if err != nil {
		slog.Error("list all submissions", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.decorate(r, subs, claims)

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": total})
}

// decorate fills the per-viewer CanView/CanRejudge flags on a page of rows.
//
// One permission query per page (not per row) keeps the status page at a fixed
// number of round trips regardless of page size.
func (h *SubmissionHandler) decorate(r *http.Request, subs []model.Submission, claims *auth.Claims) {
	if len(subs) == 0 {
		return
	}
	isAdmin := claims != nil && claims.Role == "admin"

	levels := map[string]string{}
	if claims != nil && !isAdmin {
		problemIDs := make([]string, 0, len(subs))
		for _, s := range subs {
			problemIDs = append(problemIDs, s.ProblemID)
		}
		// A lookup failure only costs the per-row actions; the listing itself is
		// already filtered, so it is not worth failing the request over.
		if got, err := h.probStore.ListAccessLevels(r.Context(), claims.UserID, problemIDs); err == nil {
			levels = got
		}
	}

	for i := range subs {
		level := levels[subs[i].ProblemID]
		canManage := isAdmin || level == "owner" || level == "co_author"
		subs[i].CanRejudge = canManage
		subs[i].CanView = canManage || (claims != nil && subs[i].UserID == claims.UserID)
	}
}

// ListByProblemManage serves GET /api/problems/{slug}/submissions/manage.
//
// This is the setter's own submission list: it names the submitter, filters by
// contest and verdict, and is the surface the rejudge actions act on.
func (h *SubmissionHandler) ListByProblemManage(w http.ResponseWriter, r *http.Request) {
	problem, err := h.probStore.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil || problem == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	if !h.canManageProblem(r, problem.ID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	q := r.URL.Query()
	offset, limit := pageParams(r)
	filter := model.SubmissionFilter{
		ContestID: q.Get("contest_id"),
		Language:  q.Get("language"),
		Status:    q.Get("status"),
		Username:  q.Get("handle"),
		UserID:    q.Get("user_id"),
	}

	subs, total, err := h.subStore.ListByProblemManage(r.Context(), problem.ID, offset, limit, filter)
	if err != nil {
		slog.Error("list problem submissions", "problem", problem.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	for i := range subs {
		subs[i].CanView = true
		subs[i].CanRejudge = true
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": total})
}

// RejudgeProblemSubmissions serves POST /api/problems/{slug}/submissions/rejudge.
//
// Body: {"submission_id": "…"} rejudges one row, {"contest_id": "…"} narrows a
// batch to one contest, {} rejudges every judged submission of the problem.
func (h *SubmissionHandler) RejudgeProblemSubmissions(w http.ResponseWriter, r *http.Request) {
	problem, err := h.probStore.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil || problem == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	if !h.canManageProblem(r, problem.ID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		SubmissionID string `json:"submission_id"`
		ContestID    string `json:"contest_id"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
	}

	if req.SubmissionID != "" {
		h.rejudgeOne(r, w, problem.ID, req.SubmissionID)
		return
	}
	h.rejudgeBatch(r, w, problem.ID, req.ContestID)
}

// rejudgeOne rejudges a single submission. Remote submissions are handed back
// to the vjudge submit worker the same way the admin rejudge does it.
func (h *SubmissionHandler) rejudgeOne(r *http.Request, w http.ResponseWriter, problemID, submissionID string) {
	sub, err := h.subStore.GetByID(r.Context(), submissionID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load submission"})
		return
	}
	if sub == nil || sub.ProblemID != problemID {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": errSubmissionNotOnProblem.Error()})
		return
	}
	if sub.Status == model.StatusPending || sub.Status == model.StatusJudging {
		respondJSON(w, http.StatusConflict, map[string]string{"error": errSubmissionInFlight.Error()})
		return
	}

	if sub.RemoteID != "" {
		h.subStore.UpdateStatus(r.Context(), sub.ID, model.StatusPending)
		h.subStore.UpdateRemoteID(r.Context(), sub.ID, "", "")
		h.subStore.UpdateBotID(r.Context(), sub.ID, "", "")
		h.recordRejudge(r, problemID, submissionID, "", 1, 1)
		respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": 1, "total": 1})
		return
	}

	if h.queue == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "judge queue not available"})
		return
	}
	if err := h.subStore.ResetForRejudge(r.Context(), []string{sub.ID}); err != nil {
		slog.Error("problem rejudge: reset failed", "id", sub.ID, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset submission"})
		return
	}
	if err := h.queue.Enqueue(r.Context(), sub.ID, PriorityNormal); err != nil {
		slog.Error("problem rejudge: enqueue failed", "id", sub.ID, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enqueue"})
		return
	}
	h.subStore.RecomputeProblemCounts(r.Context(), problemID)
	h.recordRejudge(r, problemID, submissionID, "", 1, 1)
	respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": 1, "total": 1})
}

// rejudgeBatch rejudges every judged local submission of a problem (optionally
// narrowed to one contest) at PriorityBulk, so it cannot delay live contest
// judging.
func (h *SubmissionHandler) rejudgeBatch(r *http.Request, w http.ResponseWriter, problemID, contestID string) {
	if h.queue == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "judge queue not available"})
		return
	}

	ids, err := h.subStore.ListIDsForProblemRejudge(r.Context(), problemID, model.SubmissionFilter{ContestID: contestID})
	if err != nil {
		slog.Error("problem rejudge: list failed", "problem", problemID, "contest", contestID, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list submissions"})
		return
	}
	if len(ids) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": 0, "total": 0})
		return
	}

	if err := h.subStore.ResetForRejudge(r.Context(), ids); err != nil {
		slog.Error("problem rejudge: reset failed", "problem", problemID, "count", len(ids), "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset submissions"})
		return
	}

	enqueued := 0
	for _, id := range ids {
		if err := h.queue.Enqueue(r.Context(), id, PriorityBulk); err != nil {
			slog.Error("problem rejudge: enqueue failed", "id", id, "error", err)
			continue
		}
		enqueued++
	}

	// The counters are incremented per judged row and untouched by a reset, so a
	// rejudge would double-count them. Rebuild once per batch instead.
	if err := h.subStore.RecomputeProblemCounts(r.Context(), problemID); err != nil {
		slog.Warn("problem rejudge: recount failed", "problem", problemID, "error", err)
	}

	h.recordRejudge(r, problemID, "", contestID, len(ids), enqueued)
	slog.Info("problem rejudge enqueued", "problem", problemID, "contest", contestID,
		"total", len(ids), "enqueued", enqueued)
	respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": enqueued, "total": len(ids)})
}

// recordRejudge writes an audit entry. Setters and co-authors can now rejudge
// without an admin, so the action has to be attributable after the fact.
func (h *SubmissionHandler) recordRejudge(r *http.Request, problemID, submissionID, contestID string, total, enqueued int) {
	if h.audit == nil {
		return
	}
	detail, _ := json.Marshal(map[string]interface{}{
		"submission_id": submissionID,
		"contest_id":    contestID,
		"total":         total,
		"enqueued":      enqueued,
	})
	entry := &model.AuditEntry{
		Action:     "submissions.rejudge",
		TargetType: "problem",
		TargetID:   problemID,
		Detail:     detail,
		IP:         middleware.ClientIP(r),
	}
	if claims := middleware.GetUserClaims(r); claims != nil {
		entry.ActorID = claims.UserID
		entry.ActorName = claims.Username
	}
	h.audit.Record(entry)
}
