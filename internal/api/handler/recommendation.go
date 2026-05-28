package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

type RecommendationHandler struct {
	probStore   store.ProblemStore
	ratingStore store.RatingStore
}

func NewRecommendationHandler(p store.ProblemStore, r store.RatingStore) *RecommendationHandler {
	return &RecommendationHandler{probStore: p, ratingStore: r}
}

func (h *RecommendationHandler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Default starting rating is 1200
	rating := 1200
	latest, err := h.ratingStore.GetLatestByUser(r.Context(), claims.UserID)
	if err == nil && latest != nil {
		rating = latest.NewRating
	}

	// Support query override (e.g. for setter/admin preview)
	if qRating := r.URL.Query().Get("rating"); qRating != "" {
		if val, err := strconv.Atoi(qRating); err == nil {
			rating = val
		}
	}

	resp, err := h.probStore.GetRecommendations(r.Context(), claims.UserID, rating)
	if err != nil {
		http.Error(w, "failed to get recommendations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
