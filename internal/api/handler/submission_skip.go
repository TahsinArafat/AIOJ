package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Voiding a submission.
 *
 * Marking a submission skipped is an organizer's ruling that it should not
 * count: a solution posted from a second account, a resubmit-rule breach, a
 * copied solution. The submission stops being an attempt and stops contributing
 * points, so the round is scored exactly as if it had never been made.
 *
 * This is deliberately NOT a verdict the judge can produce, and not a score
 * adjustment. The existing corrections endpoint layers a signed delta over the
 * computed standings and leaves the verdicts alone — it exists so a human can
 * fix a *number*. "Skip" is the opposite kind of decision: the number the judge
 * computed was correct, and the submission is being removed from the round
 * anyway. The two are kept apart so an audited round can still be recomputed:
 * the scoreboard reads nothing but the submissions, so voiding one is a change
 * the scorer itself understands.
 *
 * Authorization has two independent halves:
 *
 *  1. The role policy (`feature.KeySkipSubmission`, edited under Admin →
 *     Feature Gates) decides which roles may void anything at all. It is
 *     enforced by middleware on the route, before this code runs.
 *  2. The caller must also be accountable for THIS submission: they manage the
 *     problem it answers (owner/co-author) or they run the round it was made in
 *     (manager/judge). It is not enough to be a platform admin or a setter
 *     somewhere else on the site — voiding moves a public leaderboard, so the
 *     people who answer for that leaderboard are the ones who may do it.
 *
 * There are two entry points because the tab that shows the rows differs: a
 * round's submissions are read under the contest (where the round is in the
 * URL), and a problem's submissions under the problem. The check that matters —
 * half 2 above — is identical, and lives in one place so the two routes cannot
 * drift apart.
 */

// skipWindow is how much of the submission's identity the audit entry records,
// so the action stays attributable without duplicating the row.
type skipRequest struct {
	// Reason is required: a void nobody can explain later is worse than the
	// submission it removed. Mirrors the corrections endpoint, which requires one
	// for the same reason.
	Reason string `json:"reason"`
}

// canSkip reports whether the caller is accountable for this submission.
//
// A skipped submission is checked first: it is already void, so there is
// nothing to authorize and reporting it as such is more useful than a 403 that
// suggests a permission problem.
func (h *SubmissionHandler) canSkip(r *http.Request, sub *model.Submission) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		return false
	}
	// A generated onsite account is a competitor, never an organizer. The role
	// policy cannot save this: such an account can be minted with a setter-ish
	// role, and it must still not be able to void anybody's submission.
	if isArchiveBlocked(claims) {
		return false
	}
	if claims.Role == model.RoleAdmin {
		return true
	}
	// The problem it answers: its owner or a co-author.
	if h.probStore.HasAccess(r.Context(), sub.ProblemID, claims.UserID, "owner", "co_author") {
		return true
	}
	// Or the round it was made in, for a round that has one.
	if sub.ContestID != "" && h.contestStore != nil {
		if h.contestStore.HasAccess(r.Context(), sub.ContestID, claims.UserID, "manager", "judge") {
			return true
		}
	}
	return false
}

// loadForSkip resolves the submission named in the URL and applies the shared
// half of the authorization. It writes the response and returns nil when the
// request cannot proceed, so both entry points fail identically.
func (h *SubmissionHandler) loadForSkip(w http.ResponseWriter, r *http.Request) *model.Submission {
	// submissionId is read first: on the contest-scoped route {id} is the
	// CONTEST, so treating it as the submission would look up the wrong row.
	id := chi.URLParam(r, "submissionId")
	if id == "" {
		id = chi.URLParam(r, "id")
	}
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil
	}
	if sub == nil {
		http.Error(w, "submission not found", http.StatusNotFound)
		return nil
	}
	// Scoped route: the submission must belong to the contest or problem in the
	// path. Without this, the permission check would be made against the wrong
	// round — a manager of the contest in the URL could void a submission made
	// in a round they do not run.
	//
	// Which param holds the contest depends on the route: the contest-scoped
	// route names it {id} (alongside {submissionId}), the problem-scoped route
	// has no contest segment at all and names the problem {slug}.
	rawContest := chi.URLParam(r, "contestId")
	if rawContest == "" && chi.URLParam(r, "submissionId") != "" {
		rawContest = chi.URLParam(r, "id")
	}
	if rawContest != "" {
		contest, cerr := resolveContest(r.Context(), rawContest, h.contestStore)
		if cerr != nil {
			respondContestLookupError(w, cerr)
			return nil
		}
		if sub.ContestID != contest.ID {
			http.Error(w, "submission not in this contest", http.StatusNotFound)
			return nil
		}
	}
	if raw := chi.URLParam(r, "slug"); raw != "" {
		problem, perr := h.probStore.GetBySlug(r.Context(), raw)
		if perr != nil || problem == nil {
			http.Error(w, "problem not found", http.StatusNotFound)
			return nil
		}
		if sub.ProblemID != problem.ID {
			http.Error(w, "submission not in this problem", http.StatusNotFound)
			return nil
		}
	}
	if !h.canSkip(r, sub) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil
	}
	return sub
}

// applySkip performs the void and records it.
func (h *SubmissionHandler) applySkip(w http.ResponseWriter, r *http.Request, sub *model.Submission) {
	var req skipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		http.Error(w, "reason is required", http.StatusBadRequest)
		return
	}

	// A submission the judge still owns cannot be voided: the worker is about to
	// write a verdict and the two would race. Reporting it as a conflict is
	// actionable (wait for it to finish); a silent no-op is not.
	if sub.Status == model.StatusPending || sub.Status == model.StatusJudging {
		http.Error(w, "submission is still being judged", http.StatusConflict)
		return
	}
	if sub.Status == model.StatusSkipped {
		http.Error(w, "submission is already skipped", http.StatusConflict)
		return
	}

	changed, err := h.subStore.SetSkipped(r.Context(), sub.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !changed {
		// The row moved under us between the read and the write (the judge
		// finished, or another organizer voided it). Same conflict either way.
		http.Error(w, "submission changed, retry", http.StatusConflict)
		return
	}

	// The verdict it earned stays on the row as the record of what the code
	// actually did; the audit entry is the human decision. Both are needed to
	// defend the call later.
	h.recordSkip(r, sub, req.Reason)
	// Voiding changes a standing, so the cached scoreboard is now wrong. Same
	// invalidation the corrections endpoint uses, for the same reason.
	if h.contestStore != nil && sub.ContestID != "" && h.scoreboardCache != nil {
		h.scoreboardCache.invalidate(sub.ContestID)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"id":     sub.ID,
		"status": model.StatusSkipped,
		// The verdict that was replaced, so the UI can say what was voided
		// rather than just "skipped".
		"was": sub.Status,
	})
}

// SkipContestSubmission serves POST /api/contests/{contestId}/submissions/{id}/skip.
func (h *SubmissionHandler) SkipContestSubmission(w http.ResponseWriter, r *http.Request) {
	sub := h.loadForSkip(w, r)
	if sub == nil {
		return
	}
	h.applySkip(w, r, sub)
}

// SkipProblemSubmission serves POST /api/problems/{slug}/submissions/{id}/skip.
func (h *SubmissionHandler) SkipProblemSubmission(w http.ResponseWriter, r *http.Request) {
	sub := h.loadForSkip(w, r)
	if sub == nil {
		return
	}
	h.applySkip(w, r, sub)
}

// recordSkip writes the audit entry. This is the only durable record of who
// voided what and why — the verdict columns are left as the judge wrote them —
// so a failed insert is worth a log line even though the void already happened.
func (h *SubmissionHandler) recordSkip(r *http.Request, sub *model.Submission, reason string) {
	if h.audit == nil {
		return
	}
	detail, _ := json.Marshal(map[string]any{
		"submission_id": sub.ID,
		"contest_id":    sub.ContestID,
		"problem_id":    sub.ProblemID,
		"user_id":       sub.UserID,
		"was":           sub.Status,
		"reason":        reason,
	})
	entry := &model.AuditEntry{
		Action:     "submissions.skip",
		TargetType: "submission",
		TargetID:   sub.ID,
		Detail:     detail,
		IP:         middleware.ClientIP(r),
	}
	if claims := middleware.GetUserClaims(r); claims != nil {
		entry.ActorID = claims.UserID
		entry.ActorName = claims.Username
	}
	h.audit.Record(entry)
}
