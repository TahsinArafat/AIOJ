package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// fakeAIModelStore satisfies store.AIModelStore for handler tests.
type fakeAIModelStore struct {
	byID map[string]*model.AIModel
}

func (f *fakeAIModelStore) List(context.Context) ([]model.AIModel, error) { return nil, nil }
func (f *fakeAIModelStore) GetByID(_ context.Context, id string) (*model.AIModel, error) {
	return f.byID[id], nil
}
func (f *fakeAIModelStore) GetDefault(context.Context) (*model.AIModel, error) { return nil, nil }
func (f *fakeAIModelStore) Create(context.Context, *model.AIModel) error       { return nil }
func (f *fakeAIModelStore) Update(context.Context, string, *model.AIModel) error {
	return nil
}
func (f *fakeAIModelStore) Delete(context.Context, string) error           { return nil }
func (f *fakeAIModelStore) SetEnabled(context.Context, string, bool) error { return nil }

var _ store.AIModelStore = (*fakeAIModelStore)(nil)

// TestAdminAIModelGetByIDRedactsAPIKey pins the security contract: the stored
// provider credential must never be returned to the browser. The response
// advertises presence via has_api_key only.
func TestAdminAIModelGetByIDRedactsAPIKey(t *testing.T) {
	store := &fakeAIModelStore{byID: map[string]*model.AIModel{
		"m1": {ID: "m1", Name: "primary", Endpoint: "https://api.example.com/v1", APIKey: "sk-super-secret", ModelName: "gpt-test"},
	}}
	h := NewAdminAIModelHandler(store)

	r := chi.NewRouter()
	r.Get("/admin/ai-models/{id}", h.GetByID)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/ai-models/m1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); contains(body, "sk-super-secret") {
		t.Fatalf("response leaked the API key: %s", body)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if value, exists := got["api_key"]; exists && value != "" {
		t.Errorf("api_key = %q, want omitted or empty", value)
	}
	if got["has_api_key"] != true {
		t.Error("has_api_key must be true so the UI can show a stored-credential indicator")
	}
}

// TestAdminAIModelGetByIDNotFound keeps the 404 path honest.
func TestAdminAIModelGetByIDNotFound(t *testing.T) {
	h := NewAdminAIModelHandler(&fakeAIModelStore{byID: map[string]*model.AIModel{}})

	r := chi.NewRouter()
	r.Get("/admin/ai-models/{id}", h.GetByID)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/ai-models/missing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
