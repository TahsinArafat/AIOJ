package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

type ContestProblemHandler struct {
	contestStore store.ContestStore
	problemStore store.ProblemStore
}

func NewContestProblemHandler(cs store.ContestStore, ps store.ProblemStore) *ContestProblemHandler {
	return &ContestProblemHandler{
		contestStore: cs,
		problemStore: ps,
	}
}

func (h *ContestProblemHandler) GetByIndex(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	index := chi.URLParam(r, "index")

	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	now := time.Now()
	claims := middleware.GetUserClaims(r)

	if !contest.Visible || now.Before(contest.StartTime) {
		if claims == nil || (claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	problem, err := h.contestStore.GetContestProblemByIndex(r.Context(), contestID, index)
	if err != nil || problem == nil {
		http.Error(w, "problem not found in contest", http.StatusNotFound)
		return
	}

	if now.After(contest.EndTime) && !problem.Visible {
		if !contest.UpsolvingEnabled {
			if claims == nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			isParticipant, _ := h.contestStore.IsParticipant(r.Context(), contestID, claims.UserID)
			if !isParticipant && claims.Role != "admin" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			respondJSON(w, http.StatusOK, map[string]interface{}{
				"problem":           problem,
				"contest":           contest,
				"can_submit":        false,
				"upsolving_disabled": true,
			})
			return
		}
	}

	canSubmit := true
	if now.After(contest.EndTime) && !problem.Visible && !contest.UpsolvingEnabled {
		canSubmit = false
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"problem":    problem,
		"contest":    contest,
		"can_submit": canSubmit,
	})
}
