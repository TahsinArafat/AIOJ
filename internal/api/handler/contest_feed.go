package handler

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

// constantTimeEqual compares two secrets without leaking their length or the
// position of the first differing byte through timing.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

/**
 * ICPC-style event feed.
 *
 * AIOJ had no contest-control-system surface at all: no event stream, no run feed,
 * no external-scorer interface. The onsite feature stops at balloons, prints and
 * batch team accounts. This is the smallest useful slice — a polled feed of a
 * contest's runs in creation order, with a cursor so a scorer never re-reads or
 * skips a run.
 *
 * It is "CCS-flavoured", NOT certified CDS conformance. A conformant feed also
 * needs scoreboard and freeze events, integer run-id mapping, contest-state
 * transitions, and a documented event-type vocabulary; shipping those half-done
 * would be worse than saying plainly what this is.
 */

// FeedEvent is one entry in the feed, shaped after the CCS JSON the scoreboard
// systems consume.
type FeedEvent struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Data struct {
		// RunID is AIOJ's submission id. A CCS client expects an integer; the
		// mapping table that would provide one is part of full conformance.
		RunID       string `json:"run_id"`
		Team        string `json:"team"`
		Username    string `json:"username,omitempty"`
		Problem     string `json:"problem"`
		Verdict     string `json:"verdict"`
		Score       int    `json:"score"`
		TimeMs      int    `json:"time_ms"`
		MemoryKB    int    `json:"memory_kb"`
		SubmittedAt string `json:"submitted_at"`
	} `json:"data"`
}

// Feed serves GET /api/contests/{id}/feed?since=&limit=
//
// Authorization is a shared operator-configured token (header X-Feed-Token or
// ?token=), not a user JWT: a scorer is not a contestant and has no account. That
// token is compared in constant time, and the route stays outside the user
// session machinery so an admin's cookie cannot be replayed into it.
func (h *ContestHandler) Feed(w http.ResponseWriter, r *http.Request) {
	if h.feedStore == nil || h.feedToken == "" {
		http.Error(w, "event feed is not configured", http.StatusServiceUnavailable)
		return
	}
	provided := r.Header.Get("X-Feed-Token")
	if provided == "" {
		provided = r.URL.Query().Get("token")
	}
	if !constantTimeEqual(provided, h.feedToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.store)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, next, err := h.feedStore.ListFeed(r.Context(), contest.ID, r.URL.Query().Get("since"), limit)
	if err != nil {
		// An invalid cursor is the client's problem, not a server fault.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	events := make([]FeedEvent, 0, len(runs)+1)
	// A contest event opens every stream, so a fresh consumer learns what it is
	// attached to before it starts receiving runs.
	if r.URL.Query().Get("since") == "" {
		open := FeedEvent{Type: "contest", ID: contest.ID}
		events = append(events, open)
	}
	for _, run := range runs {
		ev := FeedEvent{Type: "judgement", ID: run.SubmissionID}
		ev.Data.RunID = run.SubmissionID
		ev.Data.Team = run.TeamName
		ev.Data.Username = run.Username
		ev.Data.Problem = run.ProblemIndex
		ev.Data.Verdict = run.Status
		ev.Data.Score = run.Score
		ev.Data.TimeMs = run.TimeUsed
		ev.Data.MemoryKB = run.MemoryUsed
		ev.Data.SubmittedAt = run.CreatedAt.UTC().Format(time.RFC3339Nano)
		events = append(events, ev)
	}

	w.Header().Set("Content-Type", "application/json")
	respondJSON(w, http.StatusOK, map[string]any{
		"events": events,
		// next_cursor is what the client passes as ?since= on the following poll.
		"next_cursor":   next,
		"poll_after_ms": 2000,
	})
}

// feedLister is the one store method the feed needs, so the endpoint does not
// drag the whole submission store into ContestHandler.
type feedLister interface {
	ListFeed(ctx context.Context, contestID, since string, limit int) ([]postgres.FeedRun, string, error)
}
