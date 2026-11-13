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

/**
 * Manual score adjustments.
 *
 * A contest sometimes needs a correction that no verdict can express: a test
 * case turns out to be wrong, the judge crashes on a valid solution, a problem
 * is ambiguous and the setter rules that the points stand. The alternative to
 * this endpoint is a judge editing the submission's verdict by hand, which lies
 * about what the code did and destroys the evidence.
 *
 * So a correction is a signed delta layered over the computed standings. The
 * submissions keep what they earned, the format's scoring stays reproducible,
 * and the human decision is a separate row carrying a required reason. The
 * scoreboard applies the delta after scoring (see ContestHandler.Scoreboard).
 *
 * Who may do it: the round's managers and judges, not testers. A correction
 * moves a public leaderboard, so it needs the people accountable for the round.
 */
// adjustmentSubmissionReader is what the correction ledger reads: the round's
// submissions to recompute standings from, and one row for detail. The ledger
// never writes a submission — a correction is its own row.
type adjustmentSubmissionReader interface {
	GetByID(ctx context.Context, id string) (*model.Submission, error)
	ListByContest(ctx context.Context, contestID string, offset, limit int, filter model.SubmissionFilter) ([]model.Submission, int, error)
}

type ContestAdjustmentHandler struct {
	store        store.ContestAdjustmentStore
	contestStore contestStaffStore
	subStore     adjustmentSubmissionReader
	// cache is the scoreboard cache, so a correction is visible on the next load
	// rather than after the TTL — a judge who fixes a result and still sees the
	// old numbers assumes the fix failed.
	cache *scoreboardCache
}

func NewContestAdjustmentHandler(
	as store.ContestAdjustmentStore,
	cs contestStaffStore,
	subs adjustmentSubmissionReader,
	cache *scoreboardCache,
) *ContestAdjustmentHandler {
	return &ContestAdjustmentHandler{store: as, contestStore: cs, subStore: subs, cache: cache}
}

// canAdjust reports whether the caller may change this contest's standings.
func (h *ContestAdjustmentHandler) canAdjust(r *http.Request, contestID string) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		return false
	}
	if isArchiveBlocked(claims) {
		// A generated onsite account is a competitor, never a judge.
		return false
	}
	return claims.Role == model.RoleAdmin ||
		h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager", "judge")
}

func (h *ContestAdjustmentHandler) List(w http.ResponseWriter, r *http.Request) {
	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.contestStore)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	if !h.canAdjust(r, contest.ID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	items, err := h.store.ListByContest(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "failed to list adjustments", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

func (h *ContestAdjustmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.contestStore)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	if !h.canAdjust(r, contest.ID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req model.ContestAdjustmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if msg := req.Validate(); msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// A penalty can only add. A negative penalty silently improves every later
	// tie-break, which is not a correction any judge means to make — if a
	// penalty was wrong, remove the adjustment that added it.
	if req.PenaltyDelta < 0 {
		http.Error(w, "penalty_delta cannot be negative; delete the adjustment that added it instead",
			http.StatusBadRequest)
		return
	}

	// The participant must be in this round. Adjusting a non-participant would
	// write a row that no scoreboard ever shows.
	if !h.isParticipant(r, contest.ID, req.UserID) {
		http.Error(w, "that user has no submissions in this contest", http.StatusBadRequest)
		return
	}

	adjustment := &model.ContestAdjustment{
		ContestID:    contest.ID,
		UserID:       req.UserID,
		Scope:        req.Scope,
		PointDelta:   req.PointDelta,
		SolvedDelta:  req.SolvedDelta,
		PenaltyDelta: req.PenaltyDelta,
		Reason:       req.Reason,
	}

	switch req.Scope {
	case model.AdjustmentProblem:
		// The problem has to belong to this round, or the correction would point
		// at a column this scoreboard does not have.
		problemID, ok := h.contestProblemID(r, contest.ID, *req.ProblemID)
		if !ok {
			http.Error(w, "that problem is not part of this contest", http.StatusBadRequest)
			return
		}
		adjustment.ProblemID = &problemID
	case model.AdjustmentSubmission:
		sub, err := h.subStore.GetByID(r.Context(), *req.SubmissionID)
		if err != nil || sub == nil {
			http.Error(w, "submission not found", http.StatusNotFound)
			return
		}
		// Both the submission's contest and its author must match, otherwise a
		// judge could adjust round B's submission through round A, or credit a
		// correction to the wrong competitor.
		if sub.ContestID != contest.ID {
			http.Error(w, "that submission is not part of this contest", http.StatusBadRequest)
			return
		}
		if sub.UserID != req.UserID {
			http.Error(w, "that submission was not made by this participant", http.StatusBadRequest)
			return
		}
		adjustment.SubmissionID = &sub.ID
		pid := sub.ProblemID
		adjustment.ProblemID = &pid
	}

	claims := middleware.GetUserClaims(r)
	if claims != nil {
		adjustment.CreatedBy = &claims.UserID
	}

	if err := h.store.Create(r.Context(), adjustment); err != nil {
		http.Error(w, "failed to create adjustment", http.StatusInternalServerError)
		return
	}

	// The scoreboard is cached per view; a correction must be visible on the next
	// load rather than after the cache expires, or a judge who just fixed a
	// result sees the old numbers and assumes it failed.
	h.invalidateScoreboard(r)

	respondJSON(w, http.StatusCreated, adjustment)
}

func (h *ContestAdjustmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.contestStore)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	if !h.canAdjust(r, contest.ID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "adjustmentId")
	if err := h.store.Delete(r.Context(), contest.ID, id); err != nil {
		http.Error(w, "failed to delete adjustment", http.StatusInternalServerError)
		return
	}
	h.invalidateScoreboard(r)
	w.WriteHeader(http.StatusNoContent)
}

// isParticipant reports whether the user has any submission in the round.
//
// GetParticipants is the same source the scoreboard builds its rows from, so
// "is a participant" means exactly "appears on the board".
func (h *ContestAdjustmentHandler) isParticipant(r *http.Request, contestID, userID string) bool {
	if h.subStore == nil {
		return true
	}
	_, total, err := h.subStore.ListByContest(r.Context(), contestID, 0, 1,
		model.SubmissionFilter{UserID: userID})
	if err != nil {
		// Fail closed on a store error would reject a legitimate correction
		// because of a transient fault; the alternative is a harmless row that
		// no board renders. Prefer allowing it.
		return true
	}
	return total > 0
}

// contestProblemID resolves a problem id against the round, returning the
// canonical id and whether it belongs here. A correction must point at a column
// this round actually has.
func (h *ContestAdjustmentHandler) contestProblemID(r *http.Request, contestID, problemID string) (string, bool) {
	problems, err := h.contestStore.GetProblems(r.Context(), contestID)
	if err != nil {
		return "", false
	}
	for _, p := range problems {
		if p.ProblemID == problemID {
			return p.ProblemID, true
		}
	}
	return "", false
}

// invalidateScoreboard drops every cached view of this round's board.
func (h *ContestAdjustmentHandler) invalidateScoreboard(r *http.Request) {
	if h.cache == nil {
		return
	}
	contestID := chi.URLParam(r, "id")
	if c, err := h.contestStore.GetByID(r.Context(), contestID); err == nil && c != nil {
		h.cache.invalidate(c.ID)
	}
}
