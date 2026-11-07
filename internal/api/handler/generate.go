package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

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

// legacyGenerateTimeout bounds the deprecated synchronous generation call.
//
// This endpoint runs the whole AI generation inline in the HTTP request, so
// without a deadline a slow provider held the connection open indefinitely and
// tied up a server goroutine with it. The asynchronous jobs API
// (POST /api/ai/jobs) has no such problem and is the supported path.
const legacyGenerateTimeout = 5 * time.Minute

// Problem handles POST /api/generate/problem.
// It is restricted to users with the "admin" or "setter" role.
//
// Deprecated: this runs generation synchronously inside the request and has no
// sandbox review or size cap. Prefer POST /api/ai/jobs, which queues the work,
// streams progress, and validates the result before it can be applied.
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

	// Point clients at the supported path without breaking existing callers.
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", `</api/ai/jobs>; rel="successor-version"`)

	ctx, cancel := context.WithTimeout(r.Context(), legacyGenerateTimeout)
	defer cancel()

	result, err := h.svc.Generate(ctx, claims.UserID, req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			http.Error(w, "generation timed out; use POST /api/ai/jobs for long runs",
				http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "generation failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	respondJSON(w, http.StatusCreated, result)
}
