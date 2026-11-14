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
 *  2. The caller must also be accountable for THIS submission — see
 *     canGovernSubmission below.
 *
 * The route is on the submission itself (`POST /api/submissions/{id}/skip`),
 * not under a problem or a contest. Voiding is one decision about one row, and
 * it is the same decision whichever tab the organizer happens to be looking at;
 * scoping the URL by problem or by round only created two ways to say it.
 */

// skipRequest carries the ruling.
type skipRequest struct {
	// Reason is required: a void nobody can explain later is worse than the
	// submission it removed. Mirrors the corrections endpoint, which requires one
	// for the same reason.
	Reason string `json:"reason"`
}

// canGovernSubmission reports whether the caller answers for this submission,
// which is the single rule behind every organizer action on it: voiding it,
// rejudging it, or being offered either button in a listing.
//
// It is not enough to hold an organizer role somewhere: voiding moves a public
// leaderboard, so the people who answer for that leaderboard are the ones who
// may do it. That is either the problem the submission answers (its owner or a
// co-author) or the round it was made in (its manager or judge).
func (h *SubmissionHandler) canGovernSubmission(r *http.Request, sub *model.Submission) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		return false
	}
	// A generated onsite account is a competitor, never an organizer. The role
	// policy cannot save this: such an account can be minted with a setter-ish
	// role, and it must still not be able to rule on anybody's submission.
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

// SkipSubmission serves POST /api/submissions/{id}/skip.
func (h *SubmissionHandler) SkipSubmission(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subStore.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if sub == nil {
		http.Error(w, "submission not found", http.StatusNotFound)
		return
	}
	if !h.canGovernSubmission(r, sub) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

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
