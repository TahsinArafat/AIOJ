package handler

import (
	"encoding/json"
	"net/http"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/authmethod"
)

// AuthMethodsHandler serves which sign-in methods are usable, plus the admin
// switches that control them.
type AuthMethodsHandler struct {
	Methods *authmethod.Resolver
}

// Public handles GET /api/auth/methods.
//
// Public on purpose: the login and register forms are rendered before there is
// a session, and they need to know whether to show password fields and which
// provider buttons to offer. The payload is three booleans — nothing about any
// caller — so serving it anonymously leaks nothing.
func (h *AuthMethodsHandler) Public(w http.ResponseWriter, r *http.Request) {
	if h.Methods == nil {
		// The router only mounts these routes with a handler, and main.go always
		// installs a resolver. Returning a clear error beats a nil dereference
		// taking the process down if that wiring ever changes.
		http.Error(w, "sign-in methods unavailable", http.StatusServiceUnavailable)
		return
	}
	// no-store: a switch flipped by an admin must take effect on the next page
	// load, and a cached copy would make it look broken.
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, map[string]any{"data": h.Methods.Availability(r.Context())})
}

// AdminGet handles GET /api/admin/auth-methods.
func (h *AuthMethodsHandler) AdminGet(w http.ResponseWriter, r *http.Request) {
	if h.Methods == nil {
		http.Error(w, "sign-in methods unavailable", http.StatusServiceUnavailable)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": h.Methods.AdminView(r.Context())})
}

// AdminSet handles PUT /api/admin/auth-methods.
//
// The body is a partial update: a field that is absent keeps its current value.
// Decoding straight into authmethod.Settings would make every omitted field
// mean "false", so a client that only knows about a provider switch added later
// would silently close password registration.
func (h *AuthMethodsHandler) AdminSet(w http.ResponseWriter, r *http.Request) {
	if h.Methods == nil {
		http.Error(w, "sign-in methods unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		PasswordRegistration *bool `json:"password_registration"`
		Google               *bool `json:"google"`
		GitHub               *bool `json:"github"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	settings := h.Methods.Settings(r.Context())
	if body.PasswordRegistration != nil {
		settings.PasswordRegistration = *body.PasswordRegistration
	}
	if body.Google != nil {
		settings.Google = *body.Google
	}
	if body.GitHub != nil {
		settings.GitHub = *body.GitHub
	}

	var updatedBy *string
	if claims := middleware.GetUserClaims(r); claims != nil {
		updatedBy = &claims.UserID
	}

	if err := h.Methods.Save(r.Context(), settings, updatedBy); err != nil {
		http.Error(w, "failed to save sign-in methods", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": h.Methods.AdminView(r.Context())})
}
