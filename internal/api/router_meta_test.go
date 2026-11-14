package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/handler"
)

// TestRouterMountsMetaRoutes is a wiring test, not a behaviour test.
//
// nginx rewrites social-crawler requests for /problems/hello to
// /meta/problems/hello, so a route that exists in handler code but was never
// registered here is invisible: the crawler would get a bare 404 and the shared
// link would lose its title. It also pins the routing precedence — the static
// `problems` child must win over the `/meta/*` catch-all, otherwise every
// problem share would fail back to the generic site card.
func TestRouterMountsMetaRoutes(t *testing.T) {
	mux := NewRouter(Deps{
		Optional: OptionalDeps{
			Meta: &handler.MetaHandler{
				Origin: func() string { return "https://aioj0.cyou" },
				ProblemCard: func(_ context.Context, slug string) (*handler.ShareCard, error) {
					if slug != "hello" {
						return nil, nil
					}
					return &handler.ShareCard{
						Title:       "Hello",
						Description: "Print hello.",
						Path:        "/problems/hello",
						Kind:        "article",
					}, nil
				},
			},
		},
	}, nil)

	t.Run("problem card wins over the catch-all", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/problems/hello", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /meta/problems/hello = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<title>Hello — AIOJ</title>`) {
			t.Errorf("the problem's own title is missing:\n%s", body)
		}
		if !strings.Contains(body, `og:image" content="https://aioj0.cyou/og-default.png"`) {
			t.Errorf("the card has no absolute og:image:\n%s", body)
		}
	})

	t.Run("other app routes get the site card", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/contests", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /meta/contests = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<title>AIOJ — Online Judge</title>`) {
			t.Errorf("expected the site card:\n%s", body)
		}
		if !strings.Contains(body, `rel="canonical" href="https://aioj0.cyou/contests"`) {
			t.Errorf("canonical should be the app route, not /meta:\n%s", body)
		}
	})

	// nginx forwards a request for "/" as "/meta/". The home page is the most
	// shared URL of all, so the catch-all has to match the bare prefix.
	t.Run("home page prefix is served", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /meta/ = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `rel="canonical" href="https://aioj0.cyou/"`) {
			t.Errorf("home canonical missing:\n%s", rec.Body.String())
		}
	})

	// A crawler asking for a slug that does not exist (or is private) must be
	// answered 404, not with a 200 duplicate of the site card.
	t.Run("unknown problem is a 404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/problems/missing", nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET /meta/problems/missing = %d, want 404", rec.Code)
		}
	})
}
