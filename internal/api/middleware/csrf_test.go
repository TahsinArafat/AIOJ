package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRF_RejectsMissingToken(t *testing.T) {
	h := CSRF("test-secret-32-bytes-xxxxxxxxxxxxx")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/api/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestCSRF_AcceptsValidToken(t *testing.T) {
	h := CSRF("test-secret-32-bytes-xxxxxxxxxxxxx")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/api/x", nil)
	req.Header.Set("X-CSRF-Token", "abc")
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "abc"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCSRF_AllowsBearerWithoutToken(t *testing.T) {
	h := CSRF("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/api/x", nil)
	req.Header.Set("Authorization", "Bearer jwt-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("bearer request should pass, got %d", rec.Code)
	}
}

func TestCSRF_SafeMethodSetsCookie(t *testing.T) {
	h := CSRF("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/api/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "csrf" || cookies[0].Value == "" {
		t.Fatalf("expected csrf cookie, got %v", cookies)
	}
}
