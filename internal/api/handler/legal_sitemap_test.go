package handler

import (
	"strings"
	"testing"
)

func TestLegalSitemapURLs(t *testing.T) {
	urls := LegalSitemapURLs("https://aioj0.cyou/")
	if len(urls) != len(LegalPaths) {
		t.Fatalf("expected %d legal URLs, got %d", len(LegalPaths), len(urls))
	}

	seen := make(map[string]bool, len(urls))
	for _, u := range urls {
		if !strings.HasPrefix(u.Location, "https://aioj0.cyou/legal/") {
			t.Errorf("location %q is not an absolute legal URL", u.Location)
		}
		if seen[u.Location] {
			t.Errorf("duplicate location %q", u.Location)
		}
		seen[u.Location] = true
	}

	for _, p := range LegalPaths {
		if !seen["https://aioj0.cyou"+p] {
			t.Errorf("missing sitemap entry for %s", p)
		}
	}
}

func TestLegalSitemapURLs_NoOrigin(t *testing.T) {
	// Without a configured origin the sitemap cannot emit valid absolute URLs,
	// so the section is omitted rather than written as relative <loc> values.
	if urls := LegalSitemapURLs("   "); urls != nil {
		t.Fatalf("expected nil for empty origin, got %v", urls)
	}
}
