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

var errSubmissionInFlight = errors.New("submission is already queued for judging")

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

// decorate fills the per-viewer CanView/CanRejudge/CanSkip flags on a page of
// rows.
//
// The flags come from the same accountability rule the action endpoints enforce
// (canGovernSubmission), so a listing never offers an action the API would
// refuse. The permission lookups are batched: one query for every problem on
// the page, and one per distinct round (a page is usually one round), which
// keeps the status page at a fixed number of round trips regardless of page
// size.
func (h *SubmissionHandler) decorate(r *http.Request, subs []model.Submission, claims *auth.Claims) {
	if len(subs) == 0 || claims == nil {
		return
	}
	isAdmin := claims.Role == "admin"
	// A generated onsite account is a competitor, never an organizer.
	blocked := isArchiveBlocked(claims)

	levels := map[string]string{}
	if !isAdmin && !blocked {
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

	// Which rounds on this page the viewer runs, resolved once per round.
	governedRounds := map[string]bool{}
	if !isAdmin && !blocked && h.contestStore != nil {
		for _, s := range subs {
			if s.ContestID == "" {
				continue
			}
			if _, seen := governedRounds[s.ContestID]; seen {
				continue
			}
			governedRounds[s.ContestID] = h.contestStore.HasAccess(
				r.Context(), s.ContestID, claims.UserID, "manager", "judge")
		}
	}

	for i := range subs {
		level := levels[subs[i].ProblemID]
		canGovern := !blocked && (isAdmin || level == "owner" || level == "co_author" ||
			governedRounds[subs[i].ContestID])
		subs[i].CanRejudge = canGovern
		subs[i].CanSkip = canGovern
		subs[i].CanView = canGovern || subs[i].UserID == claims.UserID
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
	// canManageProblem already passed, so every row here is one this viewer
	// governs: owning the problem is one of the two halves of the rule, and the
	// per-row answer would be the same on every row.
	for i := range subs {
		subs[i].CanView = true
		subs[i].CanRejudge = true
		subs[i].CanSkip = true
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": total})
}

// rejudgeRequest names what to rejudge. It is one request shape because it is
// one action: the selector that is set decides the scope, and the rule that
// authorizes it (canGovernSubmission, or canRejudgeBatch for a batch) is the
// same rule the void endpoint uses.
type rejudgeRequest struct {
	// SubmissionID rejudges one row, at normal priority, as if it had just been
	// submitted.
	SubmissionID string `json:"submission_id"`
	// ProblemID rejudges every judged local submission of one problem.
	ProblemID string `json:"problem_id"`
	// ContestID rejudges a whole round, or narrows ProblemID to one round.
	ContestID string `json:"contest_id"`
}

// Rejudge serves POST /api/submissions/rejudge.
//
// It is the only rejudge entry point. Before this there were three — one under
// the problem, one under the admin console and one bulk route — and the round's
// own submissions tab had none, because a round spans problems and the route
// was problem-scoped. Naming the submission (or the round) instead makes the
// same call serve every tab.
func (h *SubmissionHandler) Rejudge(w http.ResponseWriter, r *http.Request) {
	var req rejudgeRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
	}

	switch {
	case req.SubmissionID != "":
		h.rejudgeOne(r, w, req.SubmissionID)
	case req.ProblemID != "" || req.ContestID != "":
		h.rejudgeBatch(r, w, req.ProblemID, req.ContestID)
	default:
		http.Error(w, "name a submission, a problem or a contest to rejudge", http.StatusBadRequest)
	}
}

// rejudgeOne rejudges a single submission. Remote submissions are handed back
// to the vjudge submit worker the same way the admin rejudge does it.
//
// The submission names itself; the problem and the round come from the row. A
// caller cannot point the action at a problem the row does not answer, because
// there is nowhere to say so.
func (h *SubmissionHandler) rejudgeOne(r *http.Request, w http.ResponseWriter, submissionID string) {
	sub, err := h.subStore.GetByID(r.Context(), submissionID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load submission"})
		return
	}
	if sub == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "submission not found"})
		return
	}
	if !h.canGovernSubmission(r, sub) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if sub.Status == model.StatusPending || sub.Status == model.StatusJudging {
		respondJSON(w, http.StatusConflict, map[string]string{"error": errSubmissionInFlight.Error()})
		return
	}
	// A void is a human ruling that the submission does not count. Re-running
	// the judge over it would overwrite that ruling with a verdict.
	if sub.Status == model.StatusSkipped {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "submission was voided and cannot be rejudged"})
		return
	}

	if sub.RemoteID != "" {
		h.subStore.UpdateStatus(r.Context(), sub.ID, model.StatusPending)
		h.subStore.UpdateRemoteID(r.Context(), sub.ID, "", "")
		h.subStore.UpdateBotID(r.Context(), sub.ID, "", "")
		h.recordRejudge(r, "submission", sub.ID, sub.ID, sub.ContestID, 1, 1)
		respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": 1, "total": 1})
		return
	}

	if h.queue == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "judge queue not available"})
		return
	}
	if err := h.subStore.ResetForRejudge(r.Context(), []string{sub.ID}); err != nil {
		slog.Error("rejudge: reset failed", "id", sub.ID, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset submission"})
		return
	}
	if err := h.queue.Enqueue(r.Context(), sub.ID, PriorityNormal); err != nil {
		slog.Error("rejudge: enqueue failed", "id", sub.ID, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enqueue"})
		return
	}
	h.subStore.RecomputeProblemCounts(r.Context(), sub.ProblemID)
	h.recordRejudge(r, "submission", sub.ID, sub.ID, sub.ContestID, 1, 1)
	respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": 1, "total": 1})
}

// canRejudgeBatch reports whether the caller may rejudge a set of rows named by
// a problem, a round, or both. It is the scope-level form of the accountability
// rule: naming the problem asks the problem's authors, naming the round asks the
// people running it.
func (h *SubmissionHandler) canRejudgeBatch(r *http.Request, problemID, contestID string) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil || isArchiveBlocked(claims) {
		return false
	}
	if claims.Role == model.RoleAdmin {
		return true
	}
	if problemID != "" && h.probStore.HasAccess(r.Context(), problemID, claims.UserID, "owner", "co_author") {
		return true
	}
	if contestID != "" && h.contestStore != nil &&
		h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager", "judge") {
		return true
	}
	return false
}

// rejudgeBatch rejudges every judged local submission of a problem, a round, or
// the two intersected, at PriorityBulk, so it cannot delay live contest judging.
func (h *SubmissionHandler) rejudgeBatch(r *http.Request, w http.ResponseWriter, problemID, contestID string) {
	if !h.canRejudgeBatch(r, problemID, contestID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.queue == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "judge queue not available"})
		return
	}

	ids, problemIDs, err := h.subStore.ListForRejudge(r.Context(), model.SubmissionFilter{
		ProblemID: problemID,
		ContestID: contestID,
	})
	if err != nil {
		slog.Error("rejudge: list failed", "problem", problemID, "contest", contestID, "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list submissions"})
		return
	}
	if len(ids) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": 0, "total": 0})
		return
	}

	if err := h.subStore.ResetForRejudge(r.Context(), ids); err != nil {
		slog.Error("rejudge: reset failed", "problem", problemID, "contest", contestID, "count", len(ids), "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset submissions"})
		return
	}

	enqueued := 0
	for _, id := range ids {
		if err := h.queue.Enqueue(r.Context(), id, PriorityBulk); err != nil {
			slog.Error("rejudge: enqueue failed", "id", id, "error", err)
			continue
		}
		enqueued++
	}

	// The counters are incremented per judged row and untouched by a reset, so a
	// rejudge would double-count them. Rebuild once per problem in the batch: a
	// round-wide rejudge touches several, and rebuilding only one would leave
	// the rest overstated.
	for _, id := range problemIDs {
		if err := h.subStore.RecomputeProblemCounts(r.Context(), id); err != nil {
			slog.Warn("rejudge: recount failed", "problem", id, "error", err)
		}
	}

	targetType, targetID := "problem", problemID
	if targetID == "" {
		targetType, targetID = "contest", contestID
	}
	h.recordRejudge(r, targetType, targetID, "", contestID, len(ids), enqueued)
	slog.Info("rejudge enqueued", "problem", problemID, "contest", contestID,
		"total", len(ids), "enqueued", enqueued)
	respondJSON(w, http.StatusOK, map[string]interface{}{"enqueued": enqueued, "total": len(ids)})
}

// recordRejudge writes an audit entry. Setters and co-authors can rejudge
// without an admin, and a round's staff can rejudge their round, so the action
// has to be attributable after the fact. targetType/targetID name what the
// operator acted on: one submission, a problem, or a round.
func (h *SubmissionHandler) recordRejudge(r *http.Request, targetType, targetID, submissionID, contestID string, total, enqueued int) {
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
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         middleware.ClientIP(r),
	}
	if claims := middleware.GetUserClaims(r); claims != nil {
		entry.ActorID = claims.UserID
		entry.ActorName = claims.Username
	}
	h.audit.Record(entry)
}
