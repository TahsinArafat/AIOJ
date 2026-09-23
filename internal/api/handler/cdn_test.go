package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCDN_Status_DisabledWhenUnconfigured(t *testing.T) {
	h := NewCDNHandler(CDNConfig{})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/cdn/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"enabled":false`) {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestCDN_Purge_NotConfigured(t *testing.T) {
	h := NewCDNHandler(CDNConfig{})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cdn/purge", strings.NewReader(`{"everything":true}`))
	rr := httptest.NewRecorder()
	h.Purge(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("code = %d, want 501", rr.Code)
	}
}

func TestCDN_Purge_RejectsEmptyAndUnsafe(t *testing.T) {
	h := NewCDNHandler(CDNConfig{ZoneID: "z", APIToken: "t", BaseURL: "http://example.invalid"})

	// empty body → 400
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cdn/purge", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	h.Purge(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty: code = %d", rr.Code)
	}

	// unsafe file URL → 400
	req = httptest.NewRequest(http.MethodPost, "/api/admin/cdn/purge",
		strings.NewReader(`{"files":["https://evil.example/x"]}`))
	// evil is actually allowed as absolute http(s) — test javascript: instead
	req = httptest.NewRequest(http.MethodPost, "/api/admin/cdn/purge",
		strings.NewReader(`{"files":["javascript:alert(1)"]}`))
	rr = httptest.NewRecorder()
	h.Purge(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unsafe: code = %d body=%s", rr.Code, rr.Body.String())
	}
}

func Test_isSafePurgeURL(t *testing.T) {
	ok := []string{"/media/a.png", "https://cdn.example/x", "http://localhost/x"}
	bad := []string{"", "//evil", "ftp://x", "javascript:alert(1)", " https://x"}
	for _, u := range ok {
		if !isSafePurgeURL(u) {
			t.Errorf("want safe: %q", u)
		}
	}
	for _, u := range bad {
		if isSafePurgeURL(u) {
			t.Errorf("want unsafe: %q", u)
		}
	}
}
