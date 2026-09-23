package seed

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tahsinarafat/aioj/internal/auth"
)

func TestSeed_defaultsAreNeverEmpty(t *testing.T) {
	cfg := fillDefaults(Config{})
	if cfg.BackendURL == "" || cfg.AdminUser == "" || cfg.AdminPass == "" || cfg.ProblemSlug == "" {
		t.Fatalf("defaults must be non-empty: %+v", cfg)
	}
	if cfg.BackendURL != defaultBackendURL || cfg.AdminUser != defaultAdminUser ||
		cfg.AdminPass != defaultAdminPass || cfg.ProblemSlug != defaultProblemSlug {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestSeed_backendURLHasNoTrailingSlash(t *testing.T) {
	for _, in := range []string{"http://backend:8080/", "http://backend:8080//", "http://backend:8080"} {
		if got := fillDefaults(Config{BackendURL: in}).BackendURL; got != "http://backend:8080" {
			t.Fatalf("fillDefaults(%q) = %q, want %q", in, got, "http://backend:8080")
		}
	}
}

func TestSeed_defaultAdminPasswordMeetsAuthPolicy(t *testing.T) {
	if err := auth.ValidatePasswordStrength(defaultAdminPass); err != nil {
		t.Fatalf("default seeder password is rejected by auth policy: %v", err)
	}
}

func TestPostJSON_SendsCSRFTokenFromCookieJar(t *testing.T) {
	var gotCookie, gotHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("csrf"); err == nil {
			gotCookie = c.Value
		}
		gotHeader = r.Header.Get("X-CSRF-Token")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(base, []*http.Cookie{{Name: "csrf", Value: "csrf-token", Path: "/"}})

	resp, err := postJSON(context.Background(), &http.Client{Jar: jar}, server.URL, map[string]string{"ok": "yes"})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotCookie != "csrf-token" || gotHeader != "csrf-token" {
		t.Fatalf("CSRF headers: cookie=%q header=%q, want both csrf-token", gotCookie, gotHeader)
	}
}

func TestSeed_buildHelloTestcases(t *testing.T) {
	b, err := buildHelloTestcases()
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("want 2 entries (1.in, 1.out), got %d", len(zr.File))
	}
	want := map[string]string{"1.in": "", "1.out": helloExpected}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		if string(buf) != want[f.Name] {
			t.Fatalf("%s = %q, want %q", f.Name, string(buf), want[f.Name])
		}
	}
}
