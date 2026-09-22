package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The original bug was not a wrong value -- it was a query that errored on a
// non-existent column, whose failure the caller swallowed, producing a 200 with
// an empty <urlset>. That is indistinguishable from "the site has no public
// content", so the endpoint looked correct while being entirely broken.
//
// These tests pin the distinction at the HTTP seam: a fetch that failed must be
// an error response, never a well-formed empty sitemap.

func TestServeSitemapReportsFetchFailure(t *testing.T) {
	// Simulates what main.go does today: gather, log, and drop the error.
	// The failure must surface in the response, not vanish.
	sections := []SitemapSection{
		{Name: "problems", URLs: nil, Err: errFakeFetch},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	ServeSitemapSections(sections, rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("a failed fetch returned HTTP 200 with body %q; "+
			"a broken query must not masquerade as an empty sitemap", rec.Body.String())
	}
}

// A genuinely empty site is still a valid sitemap and must return 200, so the
// fix must not overcorrect into erroring on legitimately empty content.
func TestServeSitemapAllowsGenuinelyEmptySite(t *testing.T) {
	sections := []SitemapSection{
		{Name: "problems", URLs: nil, Err: nil},
		{Name: "contests", URLs: nil, Err: nil},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	ServeSitemapSections(sections, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("empty site returned %d, want 200: an empty urlset is valid", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<urlset") {
		t.Errorf("expected a urlset document, got %q", rec.Body.String())
	}
}

// Partial failure must not silently drop a section either: serving a sitemap
// that omits half the site is a correctness problem a crawler cannot detect.
func TestServeSitemapPartialFailureIsReported(t *testing.T) {
	sections := []SitemapSection{
		{Name: "problems", URLs: []SitemapURL{{Location: "https://aioj.dev/problems/hello", Priority: "0.8"}}},
		{Name: "contests", URLs: nil, Err: errFakeFetch},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	ServeSitemapSections(sections, rec, req)

	if rec.Code == http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("partial fetch failure returned 200 with %q; "+
			"a section that failed must not be silently omitted", body)
	}
}

type fakeErr struct{}

func (fakeErr) Error() string { return "simulated query failure" }

var errFakeFetch error = fakeErr{}
