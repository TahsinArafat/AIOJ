package handler

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// SitemapURL is one entry in the sitemap, decoupled from the postgres row type
// so this package doesn't depend on a concrete database representation.
type SitemapURL struct {
	Location   string
	LastMod    time.Time
	ChangeFreq string
	Priority   string
}

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

type urlEntry struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// BuildSitemapXML renders the given entries as a sitemaps.org urlset document.
//
// Ported from dmoj's judge/sitemap.py. Kept as a pure function of its input so
// the XML shape can be tested without a database.
func BuildSitemapXML(groups [][]SitemapURL) ([]byte, error) {
	set := urlSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, group := range groups {
		for _, e := range group {
			entry := urlEntry{
				Loc:        e.Location,
				ChangeFreq: e.ChangeFreq,
				Priority:   e.Priority,
			}
			if !e.LastMod.IsZero() {
				entry.LastMod = e.LastMod.UTC().Format("2006-01-02")
			}
			set.URLs = append(set.URLs, entry)
		}
	}

	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

// ServeSitemap writes the assembled sitemap with crawler-appropriate headers.
func ServeSitemap(groups [][]SitemapURL, w http.ResponseWriter, r *http.Request) {
	body, err := BuildSitemapXML(groups)
	if err != nil {
		slog.Error("build sitemap failed", "error", err)
		http.Error(w, "failed to build sitemap", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	// Crawlers refetch sitemaps often; an hour of caching keeps the database
	// from being hit on every request while still picking up new problems.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(body)
}

// NormalizeOrigin trims a configured origin to scheme://host with no trailing
// slash, so concatenating a path yields exactly one slash. A bare host gets
// https:// so a misconfigured value can't emit relative <loc> URLs, which are
// invalid in the sitemap spec.
func NormalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	origin = strings.TrimSuffix(origin, "/")
	if origin == "" {
		return ""
	}
	if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
		origin = "https://" + origin
	}
	return origin
}
