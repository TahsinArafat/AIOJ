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

	// Normalize applies defaults and enforces the cross-field brief rules
	// (rating range, ICPC/subtask conflict, team size). One implementation
	// shared with any future jobs API so the rules cannot drift.
	req, err := req.Normalize()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.svc.Generate(r.Context(), claims.UserID, req)
	if err != nil {
		http.Error(w, "generation failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	respondJSON(w, http.StatusCreated, result)
}
