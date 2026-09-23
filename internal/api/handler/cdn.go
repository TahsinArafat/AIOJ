package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// CDNConfig selects the Cloudflare purge integration.
// When ZoneID and APIToken are empty, Purge is a no-op that reports disabled=true
// so the admin UI can hide or explain the button without failing.
type CDNConfig struct {
	ZoneID   string
	APIToken string
	// BaseURL is overridable for tests; production uses the Cloudflare API.
	BaseURL string
}

// CDNHandler exposes admin cache invalidation for the edge CDN (Cloudflare).
type CDNHandler struct {
	cfg CDNConfig
	// client is overridable in tests.
	client *http.Client
}

func NewCDNHandler(cfg CDNConfig) *CDNHandler {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.cloudflare.com/client/v4"
	}
	// Prefer env when fields empty (deploy without editing config.yaml).
	if cfg.ZoneID == "" {
		cfg.ZoneID = os.Getenv("CF_ZONE_ID")
	}
	if cfg.APIToken == "" {
		cfg.APIToken = os.Getenv("CF_API_TOKEN")
	}
	return &CDNHandler{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
}

type cdnPurgeRequest struct {
	// Tags purge by cache tag; Files by URL; Everything purges the whole zone.
	Everything bool     `json:"everything"`
	Files      []string `json:"files"`
	Tags       []string `json:"tags"`
	Prefixes   []string `json:"prefixes"`
}

// Status reports whether purge is configured (GET /api/admin/cdn/status).
func (h *CDNHandler) Status(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"enabled": h.cfg.ZoneID != "" && h.cfg.APIToken != "",
		"zone_id": h.cfg.ZoneID,
	})
}

// Purge invalidates edge cache (POST /api/admin/cdn/purge).
func (h *CDNHandler) Purge(w http.ResponseWriter, r *http.Request) {
	if h.cfg.ZoneID == "" || h.cfg.APIToken == "" {
		http.Error(w, "cdn not configured (set CF_ZONE_ID and CF_API_TOKEN)", http.StatusNotImplemented)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var req cdnPurgeRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}
	if !req.Everything && len(req.Files) == 0 && len(req.Tags) == 0 && len(req.Prefixes) == 0 {
		http.Error(w, "provide everything, files, tags, or prefixes", http.StatusBadRequest)
		return
	}
	// Only allow same-origin or absolute http(s) URLs for files/prefixes.
	for _, u := range append(append([]string{}, req.Files...), req.Prefixes...) {
		if !isSafePurgeURL(u) {
			http.Error(w, "invalid purge URL", http.StatusBadRequest)
			return
		}
	}

	var payload map[string]any
	switch {
	case req.Everything:
		payload = map[string]any{"purge_everything": true}
	case len(req.Files) > 0:
		payload = map[string]any{"files": req.Files}
	case len(req.Tags) > 0:
		payload = map[string]any{"tags": req.Tags}
	default:
		payload = map[string]any{"prefixes": req.Prefixes}
	}
	raw, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/zones/%s/purge_cache", h.cfg.BaseURL, h.cfg.ZoneID)
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, strings.NewReader(string(raw)))
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+h.cfg.APIToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		slog.Error("cdn purge request failed", "error", err)
		http.Error(w, "cdn purge failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		slog.Error("cdn purge rejected", "status", resp.StatusCode, "body", string(respBody))
		http.Error(w, "cdn purge rejected", http.StatusBadGateway)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "purged"})
}

// isSafePurgeURL accepts absolute http(s) URLs or site-relative paths.
// Leading/trailing whitespace is rejected so " https://…" cannot smuggle past.
func isSafePurgeURL(u string) bool {
	if strings.TrimSpace(u) != u || u == "" || strings.ContainsAny(u, " \t\n\r") {
		return false
	}
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return true
	}
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}
