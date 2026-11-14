package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// AdminAIModelHandler provides CRUD + toggle endpoints for AI model configurations.
type AdminAIModelHandler struct {
	store store.AIModelStore
}

func validStructuredMode(v string) bool {
	return v == "auto" || v == "json_schema" || v == "json_object" || v == "text_json"
}

// NewAdminAIModelHandler creates an AdminAIModelHandler.
func NewAdminAIModelHandler(s store.AIModelStore) *AdminAIModelHandler {
	return &AdminAIModelHandler{store: s}
}

// List handles GET /api/admin/ai-models.
// api_key is intentionally omitted from the list response.
func (h *AdminAIModelHandler) List(w http.ResponseWriter, r *http.Request) {
	models, err := h.store.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list AI models: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if models == nil {
		models = []model.AIModel{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": models})
}

// GetByID handles GET /api/admin/ai-models/{id}.
// Returns a safe record; credentials are never sent back to the browser.
func (h *AdminAIModelHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.Error(w, "AI model not found", http.StatusNotFound)
		return
	}
	m.APIKey = ""
	m.HasAPIKey = true
	respondJSON(w, http.StatusOK, m)
}

type aiModelRequest struct {
	Name                  string `json:"name"`
	Endpoint              string `json:"endpoint"`
	APIKey                string `json:"api_key"`
	ModelName             string `json:"model_name"`
	Enabled               bool   `json:"enabled"`
	Priority              int    `json:"priority"`
	Description           string `json:"description"`
	AllowPrivateNetwork   bool   `json:"allow_private_network"`
	StructuredOutputMode  string `json:"structured_output_mode"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	MaxOutputTokens       int    `json:"max_output_tokens"`
}

// Create handles POST /api/admin/ai-models.
func (h *AdminAIModelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req aiModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Endpoint == "" {
		http.Error(w, "endpoint is required", http.StatusBadRequest)
		return
	}
	if req.ModelName == "" {
		http.Error(w, "model_name is required", http.StatusBadRequest)
		return
	}
	if _, err := (ai.EndpointPolicy{AllowPrivate: req.AllowPrivateNetwork}).Validate(r.Context(), req.Endpoint); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.StructuredOutputMode == "" {
		req.StructuredOutputMode = "auto"
	}
	if !validStructuredMode(req.StructuredOutputMode) {
		http.Error(w, "invalid structured_output_mode", http.StatusBadRequest)
		return
	}
	if req.RequestTimeoutSeconds <= 0 {
		req.RequestTimeoutSeconds = 120
	}
	if req.MaxOutputTokens <= 0 {
		req.MaxOutputTokens = 8192
	}

	m := &model.AIModel{
		Name:                  req.Name,
		Endpoint:              req.Endpoint,
		APIKey:                req.APIKey,
		ModelName:             req.ModelName,
		Enabled:               req.Enabled,
		Priority:              req.Priority,
		Description:           req.Description,
		AllowPrivateNetwork:   req.AllowPrivateNetwork,
		StructuredOutputMode:  req.StructuredOutputMode,
		RequestTimeoutSeconds: req.RequestTimeoutSeconds,
		MaxOutputTokens:       req.MaxOutputTokens,
	}
	if err := h.store.Create(r.Context(), m); err != nil {
		if errors.Is(err, ai.ErrEncryptionNotConfigured) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to create AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	m.HasAPIKey = m.APIKey != ""
	m.APIKey = ""
	respondJSON(w, http.StatusCreated, m)
}

// Update handles PUT /api/admin/ai-models/{id}.
// Passing an empty api_key preserves the existing secret.
func (h *AdminAIModelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "AI model not found", http.StatusNotFound)
		return
	}

	var req aiModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Endpoint == "" || req.ModelName == "" {
		http.Error(w, "name, endpoint, and model_name are required", http.StatusBadRequest)
		return
	}
	if _, err := (ai.EndpointPolicy{AllowPrivate: req.AllowPrivateNetwork}).Validate(r.Context(), req.Endpoint); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.StructuredOutputMode == "" {
		req.StructuredOutputMode = "auto"
	}
	if !validStructuredMode(req.StructuredOutputMode) {
		http.Error(w, "invalid structured_output_mode", http.StatusBadRequest)
		return
	}
	if req.RequestTimeoutSeconds <= 0 {
		req.RequestTimeoutSeconds = 120
	}
	if req.MaxOutputTokens <= 0 {
		req.MaxOutputTokens = 8192
	}

	updated := &model.AIModel{
		Name:                  req.Name,
		Endpoint:              req.Endpoint,
		APIKey:                req.APIKey, // empty = keep existing (handled by SQL CASE)
		ModelName:             req.ModelName,
		Enabled:               req.Enabled,
		Priority:              req.Priority,
		Description:           req.Description,
		AllowPrivateNetwork:   req.AllowPrivateNetwork,
		StructuredOutputMode:  req.StructuredOutputMode,
		RequestTimeoutSeconds: req.RequestTimeoutSeconds,
		MaxOutputTokens:       req.MaxOutputTokens,
	}
	if err := h.store.Update(r.Context(), id, updated); err != nil {
		if errors.Is(err, ai.ErrEncryptionNotConfigured) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to update AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	updated.ID = id
	updated.HasAPIKey = req.APIKey != "" || existing.HasAPIKey
	updated.APIKey = ""
	respondJSON(w, http.StatusOK, updated)
}

// Toggle handles PUT /api/admin/ai-models/{id}/toggle.
// Flips the enabled flag on the model.
func (h *AdminAIModelHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	m, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.Error(w, "AI model not found", http.StatusNotFound)
		return
	}

	newEnabled := !m.Enabled
	if err := h.store.SetEnabled(r.Context(), id, newEnabled); err != nil {
		http.Error(w, "failed to toggle AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"id": id, "enabled": newEnabled})
}

// Delete handles DELETE /api/admin/ai-models/{id}.
func (h *AdminAIModelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "failed to delete AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestConnection handles POST /api/admin/ai-models/test.
// Sends a trivial completion to the provided endpoint and reports whether it succeeded.
//
// The 200-with-ok:false responses below are deliberate, not an oversight. This is
// a diagnostic: the request succeeded, and the answer to "does this endpoint
// work" is no. A 4xx would say the *probe* failed, which is a different question.
// The admin panel branches on res.ok to show the endpoint's own error text rather than treating the failure as a transport
// problem. Every other handler uses a 4xx; this one is the exception, and it is
// the only one.
func (h *AdminAIModelHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID                   string `json:"id"`
		Endpoint             string `json:"endpoint"`
		APIKey               string `json:"api_key"`
		ModelName            string `json:"model_name"`
		AllowPrivateNetwork  bool   `json:"allow_private_network"`
		StructuredOutputMode string `json:"structured_output_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Endpoint == "" || req.ModelName == "" {
		http.Error(w, "endpoint and model_name are required", http.StatusBadRequest)
		return
	}
	if req.APIKey == "" && req.ID != "" {
		if existing, _ := h.store.GetByID(r.Context(), req.ID); existing != nil {
			req.APIKey = existing.APIKey
		}
	}
	if _, err := (ai.EndpointPolicy{AllowPrivate: req.AllowPrivateNetwork}).Validate(r.Context(), req.Endpoint); err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	client := ai.NewWithOptions(req.Endpoint, req.APIKey, req.ModelName, ai.ClientOptions{Mode: req.StructuredOutputMode, AllowPrivateNetwork: req.AllowPrivateNetwork})
	started := time.Now()
	result, err := client.CompleteJSON(r.Context(),
		"You are a test assistant. Reply with a single word.",
		`{"reply":"ok"}`,
		nil,
	)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "mode": result.Mode, "latency_ms": time.Since(started).Milliseconds()})
}
