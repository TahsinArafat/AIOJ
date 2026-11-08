package handler

import (
	"embed"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed all:legal/*.md
var legalFS embed.FS

// legalDocs is the allowlist of embed basenames available under /api/legal/{doc}.
var legalDocs = map[string]bool{
	"terms_of_service": true,
	"privacy_policy":   true,
	"dmca":             true,
}

// LegalPaths are the public routes that render the legal documents. The
// frontend prerenders each one to static HTML at build time (see
// web/src/prerender/legalEntry.tsx) so crawlers and app-store reviewers see the
// policy text without executing JavaScript.
var LegalPaths = []string{"/legal/terms", "/legal/privacy", "/legal/dmca"}

// LegalSitemapURLs lists the legal pages for /sitemap.xml.
//
// They are static and always public, so unlike the store-backed sections they
// cannot fail. Priority is low — these pages explain the service, they are not
// what people search for — but being in the sitemap is what lets a compliance
// crawler discover them from the homepage.
func LegalSitemapURLs(origin string) []SitemapURL {
	origin = NormalizeOrigin(origin)
	if origin == "" {
		return nil
	}
	out := make([]SitemapURL, 0, len(LegalPaths))
	for _, p := range LegalPaths {
		out = append(out, SitemapURL{Location: origin + p, ChangeFreq: "yearly", Priority: "0.3"})
	}
	return out
}

type LegalHandler struct{}

// Serve returns the markdown body for one legal document.
func (h *LegalHandler) Serve(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(chi.URLParam(r, "doc"))
	if name == "" {
		http.Error(w, "doc required", http.StatusBadRequest)
		return
	}
	// Normalize and refuse anything that is not a plain allowlisted basename.
	name = strings.TrimSuffix(name, ".md")
	base := path.Base(name)
	if !legalDocs[base] || base != name {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	body, err := legalFS.ReadFile("legal/" + base + ".md")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
