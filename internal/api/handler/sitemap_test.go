package handler

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestNormalizeOrigin(t *testing.T) {
	cases := map[string]string{
		"https://aioj.dev":      "https://aioj.dev",
		"https://aioj.dev/":     "https://aioj.dev",
		"http://localhost:8081": "http://localhost:8081",
		"aioj.dev":              "https://aioj.dev",
		"  aioj.dev/  ":         "https://aioj.dev",
		"":                      "",
	}
	for in, want := range cases {
		if got := NormalizeOrigin(in); got != want {
			t.Errorf("NormalizeOrigin(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildSitemapXMLValid(t *testing.T) {
	groups := [][]SitemapURL{
		{
			{Location: "https://aioj.dev/problems/hello", LastMod: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), ChangeFreq: "weekly", Priority: "0.8"},
			{Location: "https://aioj.dev/problems/world", ChangeFreq: "weekly", Priority: "0.8"},
		},
		{
			{Location: "https://aioj.dev/contests/spring", LastMod: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC), ChangeFreq: "daily", Priority: "0.7"},
		},
	}

	body, err := BuildSitemapXML(groups)
	if err != nil {
		t.Fatalf("BuildSitemapXML returned error: %v", err)
	}
	out := string(body)

	if !strings.HasPrefix(out, "<?xml") {
		t.Error("expected an XML declaration")
	}
	// The trailing slash is required by the sitemap spec; crawlers reject a
	// urlset without it.
	if !strings.Contains(out, `xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`) {
		t.Error("expected the sitemap namespace")
	}
	if n := strings.Count(out, "<url>"); n != 3 {
		t.Errorf("got %d <url> entries, want 3", n)
	}
	// A missing LastMod must be omitted, not emitted as a zero date.
	if strings.Contains(out, "0001-01-01") {
		t.Error("zero LastMod was serialised; it should be omitted")
	}
	if !strings.Contains(out, "<lastmod>2026-01-02</lastmod>") {
		t.Error("expected the formatted lastmod date")
	}

	// Well-formedness is the property that actually matters to a crawler.
	var parsed struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("emitted sitemap is not well-formed XML: %v", err)
	}
	if len(parsed.URLs) != 3 {
		t.Fatalf("parsed %d urls, want 3", len(parsed.URLs))
	}
	for _, u := range parsed.URLs {
		if !strings.HasPrefix(u.Loc, "https://") {
			t.Errorf("loc %q is not absolute; sitemap spec requires absolute URLs", u.Loc)
		}
	}
}

// Escaping matters because slugs come from user input.
func TestBuildSitemapXMLEscapesAmpersand(t *testing.T) {
	body, err := BuildSitemapXML([][]SitemapURL{{
		{Location: "https://aioj.dev/problems/a&b", Priority: "0.8"},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(body), "a&b") {
		t.Error("raw ampersand emitted; XML requires &amp;")
	}
	if err := xml.Unmarshal(body, new(struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	})); err != nil {
		t.Fatalf("sitemap with ampersand is not well-formed: %v", err)
	}
}

func TestBuildSitemapXMLEmpty(t *testing.T) {
	body, err := BuildSitemapXML(nil)
	if err != nil {
		t.Fatalf("empty sitemap should not error: %v", err)
	}
	// An empty but valid urlset is correct for a site with no public content;
	// returning an error would 500 a legitimately empty sitemap.
	if err := xml.Unmarshal(body, new(any)); err != nil {
		t.Fatalf("empty sitemap is not well-formed: %v", err)
	}
}
