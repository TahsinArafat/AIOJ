package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// AdminAIModelHandler provides CRUD + toggle endpoints for AI model configurations.
type AdminAIModelHandler struct {
	store store.AIModelStore
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
// Returns the full record including the api_key.
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
	respondJSON(w, http.StatusOK, m)
}

type aiModelRequest struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	APIKey      string `json:"api_key"`
	ModelName   string `json:"model_name"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	Description string `json:"description"`
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

	m := &model.AIModel{
		Name:        req.Name,
		Endpoint:    req.Endpoint,
		APIKey:      req.APIKey,
		ModelName:   req.ModelName,
		Enabled:     req.Enabled,
		Priority:    req.Priority,
		Description: req.Description,
	}
	if err := h.store.Create(r.Context(), m); err != nil {
		http.Error(w, "failed to create AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
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

	updated := &model.AIModel{
		Name:        req.Name,
		Endpoint:    req.Endpoint,
		APIKey:      req.APIKey, // empty = keep existing (handled by SQL CASE)
		ModelName:   req.ModelName,
		Enabled:     req.Enabled,
		Priority:    req.Priority,
		Description: req.Description,
	}
	if err := h.store.Update(r.Context(), id, updated); err != nil {
		http.Error(w, "failed to update AI model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	updated.ID = id
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
func (h *AdminAIModelHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint  string `json:"endpoint"`
		APIKey    string `json:"api_key"`
		ModelName string `json:"model_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Endpoint == "" || req.ModelName == "" {
		http.Error(w, "endpoint and model_name are required", http.StatusBadRequest)
		return
	}

	client := ai.New(req.Endpoint, req.APIKey, req.ModelName)
	_, err := client.Complete(r.Context(),
		"You are a test assistant. Reply with a single word.",
		`{"reply":"ok"}`,
	)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}
