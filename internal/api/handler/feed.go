package handler

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Atom feed for recently added problems.
//
// Ported from dmoj's judge/feed.py (ProblemFeed / AtomProblemFeed), which
// publishes the 25 most recent public problems. Atom is used rather than RSS
// 2.0, matching dmoj's AtomProblemFeed, and because Atom requires an explicit
// updated timestamp and stable ids.
//
// The feed reflects the same visibility rule as the problems API
// (problems.visible), so it cannot leak a problem the site hides.

// FeedItem is one entry, decoupled from the store row type.
type FeedItem struct {
	Title   string
	Link    string
	ID      string
	Summary string
	Updated time.Time
}

type atomFeed struct {
	XMLName  xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle,omitempty"`
	ID       string      `xml:"id"`
	Link     []atomLink  `xml:"link"`
	Updated  string      `xml:"updated"`
	Author   *atomAuthor `xml:"author,omitempty"`
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomEntry struct {
	Title   string    `xml:"title"`
	Link    atomLink  `xml:"link"`
	ID      string    `xml:"id"`
	Updated string    `xml:"updated"`
	Summary *atomText `xml:"summary,omitempty"`
}

type atomText struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// BuildAtomFeed renders the entries as an Atom 1.0 document.
//
// A pure function of its input so the XML can be tested without a database.
// FeedItem fields are sanitised because problem titles are user-controlled and
// an XML-invalid character would otherwise produce a document no reader can
// parse.
func BuildAtomFeed(feedTitle, feedSubtitle, feedID, selfLink string, items []FeedItem, now time.Time) ([]byte, error) {
	feed := atomFeed{
		Title:    sanitizeXMLText(feedTitle),
		Subtitle: sanitizeXMLText(feedSubtitle),
		ID:       feedID,
		Updated:  now.UTC().Format(time.RFC3339),
		Author:   &atomAuthor{Name: "AIOJ"},
	}
	if selfLink != "" {
		feed.Link = append(feed.Link, atomLink{Href: selfLink, Rel: "self"})
	}
	if feedID != "" {
		feed.Link = append(feed.Link, atomLink{Href: feedID})
	}

	for _, it := range items {
		// Atom requires every entry to carry an updated timestamp; RFC3339
		// cannot represent the zero time, so fall back to the feed time.
		updated := it.Updated
		if updated.IsZero() {
			updated = now
		}
		entry := atomEntry{
			Title:   sanitizeXMLText(it.Title),
			Link:    atomLink{Href: it.Link},
			ID:      it.ID,
			Updated: updated.UTC().Format(time.RFC3339),
		}
		if it.Summary != "" {
			// html, not text: statements contain markdown-rendered markup and
			// readers should render it rather than show raw tags.
			entry.Summary = &atomText{Type: "html", Value: sanitizeXMLText(it.Summary)}
		}
		feed.Entries = append(feed.Entries, entry)
	}

	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

// sanitizeXMLText strips characters XML 1.0 cannot represent.
//
// Control characters (except tab/newline/carriage return) are illegal in XML 1.0
// even when escaped, so escaping is not an option -- they must be removed or the
// document is unparseable by any conforming reader.
func sanitizeXMLText(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == 0x09 || r == 0x0A || r == 0x0D:
			b.WriteRune(r)
		case r >= 0x20 && r <= 0xD7FF:
			b.WriteRune(r)
		case r >= 0xE000 && r <= 0xFFFD:
			b.WriteRune(r)
		case r >= 0x10000 && r <= 0x10FFFF:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ServeAtomFeed writes the feed with reader-appropriate headers.
//
// Like the sitemap, a failed fetch must not be served as a valid empty feed: an
// empty feed tells subscribers nothing new has happened, which is a lie when the
// query actually failed.
func ServeAtomFeed(items []FeedItem, fetchErr error, feedTitle, feedSubtitle, feedID, selfLink string, w http.ResponseWriter, r *http.Request) {
	if fetchErr != nil {
		slog.Error("feed fetch failed", "feed", feedTitle, "error", fetchErr)
		http.Error(w, "failed to build feed", http.StatusInternalServerError)
		return
	}

	body, err := BuildAtomFeed(feedTitle, feedSubtitle, feedID, selfLink, items, time.Now())
	if err != nil {
		slog.Error("build feed failed", "error", err)
		http.Error(w, "failed to build feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	// Feeds change when content is added; a short cache keeps readers from
	// hammering the database while staying reasonably fresh.
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = w.Write(body)
}
