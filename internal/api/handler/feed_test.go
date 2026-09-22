package handler

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var feedNow = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

func sampleItems() []FeedItem {
	return []FeedItem{
		{
			Title:   "Hello, AIOJ!",
			Link:    "http://localhost:8081/problems/hello",
			ID:      "http://localhost:8081/problems/hello",
			Summary: "Print <strong>Hello</strong>",
			Updated: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		},
	}
}

func TestBuildAtomFeedIsWellFormed(t *testing.T) {
	body, err := BuildAtomFeed("AIOJ Problems", "Latest problems", "http://localhost:8081/feed/problems.atom", "http://localhost:8081/feed/problems.atom", sampleItems(), feedNow)
	if err != nil {
		t.Fatalf("BuildAtomFeed error: %v", err)
	}

	var parsed struct {
		XMLName xml.Name `xml:"feed"`
		Title   string   `xml:"title"`
		Updated string   `xml:"updated"`
		Entries []struct {
			Title   string `xml:"title"`
			ID      string `xml:"id"`
			Updated string `xml:"updated"`
			Link    struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("emitted feed is not well-formed XML: %v", err)
	}
	if parsed.XMLName.Space != "http://www.w3.org/2005/Atom" {
		t.Errorf("wrong namespace %q; Atom readers require the 2005 Atom namespace", parsed.XMLName.Space)
	}
	if len(parsed.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(parsed.Entries))
	}
	// Atom requires id and updated on every entry; a reader rejects the feed
	// outright if either is missing.
	if parsed.Entries[0].ID == "" {
		t.Error("entry is missing the required id element")
	}
	if parsed.Entries[0].Updated == "" {
		t.Error("entry is missing the required updated element")
	}
}

// A zero timestamp cannot be formatted as RFC3339 (it renders as year 1, which
// readers reject), so it must fall back to the feed time.
func TestBuildAtomFeedZeroUpdatedDoesNotEmitYearOne(t *testing.T) {
	items := []FeedItem{{Title: "no date", Link: "http://x/p", ID: "http://x/p"}}
	body, err := BuildAtomFeed("t", "s", "http://x/feed", "", items, feedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(body)
	if strings.Contains(out, "0001-01-01") {
		t.Error("zero time was serialised; RFC3339 cannot represent it and readers reject it")
	}
	if !strings.Contains(out, feedNow.Format(time.RFC3339)) {
		t.Errorf("expected the fallback timestamp %s in the output", feedNow.Format(time.RFC3339))
	}
}

// Problem titles are user-controlled, so control characters must not reach the
// document: XML 1.0 cannot represent them even escaped.
func TestBuildAtomFeedStripsIllegalControlChars(t *testing.T) {
	items := []FeedItem{{Title: "bad\x00title\x07here", Link: "http://x/p", ID: "http://x/p", Updated: feedNow}}
	body, err := BuildAtomFeed("t", "s", "http://x/feed", "", items, feedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.ContainsRune(string(body), 0x00) || strings.ContainsRune(string(body), 0x07) {
		t.Error("illegal control characters survived into the XML")
	}
	if err := xml.Unmarshal(body, new(any)); err != nil {
		t.Fatalf("feed with control chars is not well-formed: %v", err)
	}
	if !strings.Contains(string(body), "badtitlehere") {
		t.Error("expected the legal characters to be preserved around the stripped ones")
	}
}

func TestBuildAtomFeedEscapesMarkup(t *testing.T) {
	items := []FeedItem{{Title: "a & b <script>", Link: "http://x/p", ID: "http://x/p", Updated: feedNow}}
	body, err := BuildAtomFeed("t", "s", "http://x/feed", "", items, feedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(body), "<script>") {
		t.Error("raw markup emitted; it must be escaped")
	}
	if err := xml.Unmarshal(body, new(any)); err != nil {
		t.Fatalf("not well-formed: %v", err)
	}
}

// Same failure mode the sitemap had: a broken query must not be served as a
// valid empty feed, which would tell subscribers nothing new exists.
func TestServeAtomFeedReportsFetchFailure(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/feed/problems.atom", nil)
	ServeAtomFeed(nil, errFakeFetch, "t", "s", "http://x/feed", "", rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("failed fetch returned 200 with %q; a broken query must not look like an empty feed", rec.Body.String())
	}
}

// An empty feed is legitimate (a new site has no problems) and must stay 200.
func TestServeAtomFeedAllowsEmptyFeed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/feed/problems.atom", nil)
	ServeAtomFeed(nil, nil, "t", "s", "http://x/feed", "", rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("empty feed returned %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "atom+xml") {
		t.Errorf("Content-Type = %q, want an atom+xml type", ct)
	}
}
