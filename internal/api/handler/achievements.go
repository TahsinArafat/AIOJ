package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

// AchievementHandler serves earned badges for a user.
type AchievementHandler struct {
	Achievements store.AchievementStore
}

func NewAchievementHandler(a store.AchievementStore) *AchievementHandler {
	return &AchievementHandler{Achievements: a}
}

// ListByUser: GET /api/users/{username}/achievements — needs username→id via users store.
// For MVP we award/list by path username resolved through Users store if available;
// otherwise list by path param treated as user id when UUID-shaped.
func (h *AchievementHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "userId required", http.StatusBadRequest)
		return
	}
	items, err := h.Achievements.ListForUser(r.Context(), userID)
	if err != nil {
		slog.Error("list achievements", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}

// Award is admin/test only: POST /api/users/{userId}/achievements/{code}
func (h *AchievementHandler) Award(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := chi.URLParam(r, "userId")
	code := chi.URLParam(r, "code")
	// Non-admins may only award to self (self-serve for tests; real badges come from AC path).
	if claims.Role != "admin" && userID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ok, err := h.Achievements.Award(r.Context(), userID, code)
	if err != nil {
		http.Error(w, "award failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"awarded": ok})
}
