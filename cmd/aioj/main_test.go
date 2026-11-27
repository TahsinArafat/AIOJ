package main

import (
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// The card resolver only claims the exact two-segment detail route. A prefix
// match would hand the scoreboard and per-problem routes the contest's own
// canonical URL, telling crawlers those pages are duplicates of the round.
func TestContestPathID(t *testing.T) {
	cases := []struct {
		path string
		want string
		ok   bool
	}{
		{"/contests/aioj-round-1", "aioj-round-1", true},
		{"/contests/42", "42", true},
		{"/contests/", "", false},
		{"/contests", "", false},
		{"/contests/aioj-round-1/scoreboard", "", false},
		{"/contests/aioj-round-1/plagiarism", "", false},
		{"/contests/aioj-round-1/problem/A", "", false},
		{"/problems/hello", "", false},
		{"/", "", false},
	}
	for _, c := range cases {
		got, ok := contestPathID(c.path)
		if got != c.want || ok != c.ok {
			t.Errorf("contestPathID(%q) = (%q, %v), want (%q, %v)", c.path, got, ok, c.want, c.ok)
		}
	}
}

func TestContestShareCard(t *testing.T) {
	start := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	c := &model.Contest{
		Title:     "AIOJ Round 1",
		Slug:      "aioj-round-1",
		Format:    "acm",
		Division:  model.Division3,
		StartTime: start,
		EndTime:   start.Add(2*time.Hour + 30*time.Minute),
	}

	card := contestShareCard(c)
	if card.Title != "AIOJ Round 1" {
		t.Errorf("Title = %q", card.Title)
	}
	if card.Path != "/contests/aioj-round-1" {
		t.Errorf("Path = %q, want the slug route", card.Path)
	}
	want := []string{"Div. 3", "ACM", "2h 30m"}
	if len(card.Facts) != len(want) {
		t.Fatalf("Facts = %v, want %v", card.Facts, want)
	}
	for i := range want {
		if card.Facts[i] != want[i] {
			t.Errorf("Facts[%d] = %q, want %q", i, card.Facts[i], want[i])
		}
	}

	// A round with no slug still needs a canonical path; DisplayID is the
	// fallback the store itself resolves.
	c.Slug = ""
	c.DisplayID = 42
	if got := contestShareCard(c).Path; got != "/contests/42" {
		t.Errorf("Path without a slug = %q, want /contests/42", got)
	}
}

func TestFormatCardDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Minute, "45m"},
		{6 * time.Hour, "6h"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
	}
	for _, c := range cases {
		if got := formatCardDuration(c.d); got != c.want {
			t.Errorf("formatCardDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
