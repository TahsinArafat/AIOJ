package handler

import (
	"encoding/json"
	"net/http"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/generate"
)

// GenerateHandler handles AI-powered problem generation requests.
type GenerateHandler struct {
	svc *generate.Service
}

// NewGenerateHandler creates a GenerateHandler.
func NewGenerateHandler(svc *generate.Service) *GenerateHandler {
	return &GenerateHandler{svc: svc}
}

// Problem handles POST /api/generate/problem.
// It is restricted to users with the "admin" or "setter" role.
// The handler validates the request, delegates to generate.Service,
// and returns the new problem slug + IDs so the caller can navigate to it.
func (h *GenerateHandler) Problem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req generate.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields.
	if len(req.Tags) == 0 {
		http.Error(w, "at least one tag is required", http.StatusBadRequest)
		return
	}
	if req.Rating < 800 || req.Rating > 3500 {
		http.Error(w, "rating must be between 800 and 3500", http.StatusBadRequest)
		return
	}
	if req.TestCaseCount < 1 || req.TestCaseCount > 100 {
		http.Error(w, "testcase_count must be between 1 and 100", http.StatusBadRequest)
		return
	}
	if req.ProblemType == "" {
		req.ProblemType = "standard"
	}

	result, err := h.svc.Generate(r.Context(), claims.UserID, req)
	if err != nil {
		http.Error(w, "generation failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	respondJSON(w, http.StatusCreated, result)
}
