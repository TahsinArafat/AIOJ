package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The error response shapes the SPA reads.
 *
 * The handlers grew two conventions side by side: 1191 `http.Error` calls
 * producing a plain-text body, and 334 `respondJSON` calls producing JSON. Which
 * one a caller got depended on which handler it hit, so the same failure looked
 * different on two endpoints, and a client that assumed JSON got a parse error
 * where it expected a message.
 *
 * The SPA reads `error`, then `message`, then falls back to the raw text
 * (web/src/lib/api.ts throwForStatus), so both conventions work for it. That is
 * why standardising on JSON is not a breaking change — the client already handles
 * the text form, and JSON is what the typed client expects. The 1191 http.Error
 * sites are left as they are: converting them is a large diff with no behavioural
 * benefit, and the client handles both. What this file adds is the shared shape
 * for the responses that carry more than a message.
 */

// errorBody is the response shape for a refusal.
//
// A struct rather than a map because the fields are a contract with the SPA, and
// a map silently accepts a misspelled key. omitempty on the extras keeps the body
// to the keys that mean something for this particular refusal.
type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
	// ScoreboardPublic tells the standings page this round publishes its board,
	// so a locked round can still be viewed. The SPA reads it off the error body
	// to decide whether to offer that prompt.
	ScoreboardPublic bool `json:"scoreboard_public,omitempty"`
	// PublishedSurfaces lists every surface this caller may follow a link to,
	// comma-separated. Generalises ScoreboardPublic: naming only the board went
	// stale the first time a round published its booklet instead.
	PublishedSurfaces string `json:"published_surfaces,omitempty"`
}

// respondErrorBody writes a refusal in the shape the SPA reads.
func respondErrorBody(w http.ResponseWriter, status int, body errorBody) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// contestPasswordRequired is the refusal for a round the caller has not unlocked.
//
// One function because there were two copies of this body in contest_access.go,
// built field by field, and they had already drifted in what they included. The
// fields are derived from the round rather than passed in, so a new public surface
// is reported everywhere at once.
func contestPasswordRequired(w http.ResponseWriter, c *model.Contest) {
	body := errorBody{
		Error: "password_required",
		Code:  "password_required",
	}
	// The board is named separately from the general list so an existing client
	// keeps working when the board is what was published.
	if c.AccessConfig.Audience(model.SurfaceStandings) == model.AccessPublic {
		body.ScoreboardPublic = true
	}
	if public := c.PublicSurfaces(); len(public) > 0 {
		body.PublishedSurfaces = strings.Join(public, ",")
	}
	respondErrorBody(w, http.StatusForbidden, body)
}
