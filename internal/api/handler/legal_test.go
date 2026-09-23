package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func legalRouter() http.Handler {
	h := &LegalHandler{}
	r := chi.NewRouter()
	r.Get("/api/legal/{doc}", h.Serve)
	return r
}

func TestLegal_ServeKnownDocs(t *testing.T) {
	for _, doc := range []string{"terms_of_service", "privacy_policy", "dmca"} {
		req := httptest.NewRequest("GET", "/api/legal/"+doc, nil)
		rec := httptest.NewRecorder()
		legalRouter().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status %d", doc, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
			t.Errorf("%s content-type %q", doc, ct)
		}
		if !strings.Contains(rec.Body.String(), "AIOJ") {
			t.Errorf("%s body missing title", doc)
		}
	}
}

func TestLegal_RejectsUnknownAndTraversal(t *testing.T) {
	for _, doc := range []string{"missing", "..%2fterms_of_service", "a/b"} {
		req := httptest.NewRequest("GET", "/api/legal/"+doc, nil)
		rec := httptest.NewRecorder()
		legalRouter().ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("%s: expected non-200, got %d", doc, rec.Code)
		}
	}
}
