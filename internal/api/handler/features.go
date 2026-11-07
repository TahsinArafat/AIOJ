package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
	"github.com/tahsinarafat/aioj/internal/model"
)

// FeaturesHandler serves the feature registry: the resolved decisions the SPA
// boots from, plus the admin CRUD that writes policies.
type FeaturesHandler struct {
	resolver *feature.Resolver
	store    feature.SettingsStore
}

// NewFeaturesHandler builds the handler. store is the same system-settings
// store the resolver reads, kept here so writes can be audited by the existing
// admin audit middleware without a second abstraction.
func NewFeaturesHandler(resolver *feature.Resolver, store feature.SettingsStore) *FeaturesHandler {
	return &FeaturesHandler{resolver: resolver, store: store}
}

// List handles GET /api/features.
//
// Public on purpose: the SPA needs to know which navigation entries and routes
// exist before it can render, including for a signed-out visitor. It exposes
// feature keys and the effective policy — nothing user-specific — and the
// per-caller verdict is what the client actually uses.
func (h *FeaturesHandler) List(w http.ResponseWriter, r *http.Request) {
	principal := middleware.PrincipalFromRequest(r)
	snapshot, err := h.resolver.Snapshot(r.Context(), principal)
	if err != nil {
		http.Error(w, "failed to resolve features", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, map[string]any{"data": snapshot})
}

// AdminList handles GET /api/admin/features.
func (h *FeaturesHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	stored, err := h.resolver.Policies(r.Context())
	if err != nil {
		http.Error(w, "failed to load feature policies", http.StatusInternalServerError)
		return
	}

	type entry struct {
		feature.Feature
		Policy     feature.Policy `json:"policy"`
		Customized bool           `json:"customized"`
	}

	all := feature.All()
	out := make([]entry, 0, len(all))
	for _, f := range all {
		policy := feature.Normalize(f.Default)
		customized := false
		if p, ok := stored[f.Key]; ok {
			policy = feature.Normalize(p)
			customized = !feature.Equal(policy, f.Default)
		}
		out = append(out, entry{Feature: f, Policy: policy, Customized: customized})
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": out})
}

// AdminSet handles PUT /api/admin/features/{key}.
func (h *FeaturesHandler) AdminSet(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	f, ok := feature.Lookup(key)
	if !ok {
		http.Error(w, "unknown feature", http.StatusNotFound)
		return
	}

	var policy feature.Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := feature.Validate(policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	policy = feature.Normalize(policy)

	value, err := json.Marshal(policy)
	if err != nil {
		http.Error(w, "failed to encode policy", http.StatusInternalServerError)
		return
	}

	setting := &model.SystemSetting{
		Key:         feature.SettingKey(key),
		Value:       value,
		Description: "Feature gate: " + f.Label,
	}
	if claims := middleware.GetUserClaims(r); claims != nil {
		setting.UpdatedBy = &claims.UserID
	}
	if err := h.store.Set(r.Context(), setting); err != nil {
		http.Error(w, "failed to save policy", http.StatusInternalServerError)
		return
	}

	// Same process serves the next request, so apply the change immediately
	// rather than waiting out the cache TTL.
	h.resolver.Invalidate()

	respondJSON(w, http.StatusOK, map[string]any{
		"key":      key,
		"policy":   policy,
		"applied":  true,
		"settings": setting,
	})
}

// AdminReset handles DELETE /api/admin/features/{key}: back to the registry
// default.
func (h *FeaturesHandler) AdminReset(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	f, ok := feature.Lookup(key)
	if !ok {
		http.Error(w, "unknown feature", http.StatusNotFound)
		return
	}
	if err := h.store.Delete(r.Context(), feature.SettingKey(key)); err != nil {
		http.Error(w, "failed to reset policy", http.StatusInternalServerError)
		return
	}
	h.resolver.Invalidate()

	respondJSON(w, http.StatusOK, map[string]any{
		"key":     key,
		"policy":  feature.Normalize(f.Default),
		"reset":   true,
		"applied": true,
	})
}
