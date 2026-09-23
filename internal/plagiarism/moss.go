package plagiarism

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// MossClient submits code to Stanford MOSS when configured.
// Without MOSS_USER / MOSS_LANG env, Enabled() is false and Submit is a no-op
// that returns ErrNotConfigured — local LCS engine remains the default.
type MossClient struct {
	User string
	Lang string // c, java, python, cpp, …
	Base string // default https://plagiarism.dimacs.rutgers.edu is unofficial;
	// official is moss.stanford.edu via the moss script protocol.
	// We use the simple HTTP wrapper many OJs run as a sidecar:
	// POST /moss { files: [{name, code}], lang } → { url }
	HTTP *http.Client
}

// ErrNotConfigured when MOSS_USER is empty.
var ErrNotConfigured = fmt.Errorf("moss not configured")

func NewMossFromEnv() *MossClient {
	return &MossClient{
		User: os.Getenv("MOSS_USER"),
		Lang: os.Getenv("MOSS_LANG"),
		Base: os.Getenv("MOSS_URL"),
		HTTP: &http.Client{Timeout: 60 * time.Second},
	}
}

// Enabled reports whether external MOSS is available.
func (c *MossClient) Enabled() bool {
	return c != nil && c.User != "" && c.Base != ""
}

// MossFile is one submission file.
type MossFile struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type mossRequest struct {
	User  string     `json:"user"`
	Lang  string     `json:"lang"`
	Files []MossFile `json:"files"`
}

type mossResponse struct {
	URL string `json:"url"`
}

// Submit posts files to the MOSS sidecar (or compatible endpoint) and returns
// the result URL. Returns ErrNotConfigured when not enabled.
func (c *MossClient) Submit(ctx context.Context, files []MossFile) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	body, err := json.Marshal(mossRequest{User: c.User, Lang: c.Lang, Files: files})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("moss request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("moss status %d: %s", resp.StatusCode, string(raw))
	}
	var out mossResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", fmt.Errorf("moss empty url")
	}
	return out.URL, nil
}
