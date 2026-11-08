package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func renderCard(t *testing.T, card *ShareCard, origin string) string {
	t.Helper()
	body, err := RenderMetaPage(card, origin)
	if err != nil {
		t.Fatalf("RenderMetaPage returned error: %v", err)
	}
	return string(body)
}

// The whole point of the crawler page: a problem link must carry the problem's
// own title, not the SPA shell's "AIOJ — Online Judge".
func TestRenderMetaPageProblemCarriesEntityTitle(t *testing.T) {
	out := renderCard(t, &ShareCard{
		Title:       "Hello",
		Description: "Print hello to standard output.",
		Path:        "/problems/hello",
		Kind:        "article",
		Facts:       []string{"Easy", "1.0 s", "256 MB"},
	}, "https://aioj0.cyou")

	for _, want := range []string{
		`<title>Hello — AIOJ</title>`,
		`<meta property="og:title" content="Hello" />`,
		`<meta name="twitter:title" content="Hello" />`,
		`<meta property="og:url" content="https://aioj0.cyou/problems/hello" />`,
		`<link rel="canonical" href="https://aioj0.cyou/problems/hello" />`,
		`<meta property="og:image" content="https://aioj0.cyou/og-default.png" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
		`<meta property="og:type" content="article" />`,
		// The preview body leads with the facts a solver scans for.
		`<meta property="og:description" content="Easy · 1.0 s · 256 MB — Print hello to standard output." />`,
		`Print hello to standard output.`,
		`<li>Easy</li>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// A problem title is author-supplied. It must not be able to break out of the
// attribute or inject a script into the head.
func TestRenderMetaPageEscapesAttributeValues(t *testing.T) {
	out := renderCard(t, &ShareCard{
		Title:       `He said "hi"><script>alert(1)</script>`,
		Description: `a & b < c`,
		Path:        "/problems/evil",
	}, "https://aioj0.cyou")

	if strings.Contains(out, "<script>") {
		t.Errorf("unescaped script in output:\n%s", out)
	}
	if strings.Contains(out, `content="He said "`) {
		t.Errorf("title broke out of the attribute:\n%s", out)
	}
	if !strings.Contains(out, "a &amp; b &lt; c") {
		t.Errorf("description was not escaped:\n%s", out)
	}
}

func TestRenderMetaPageSiteCardWithoutEntity(t *testing.T) {
	out := renderCard(t, nil, "https://aioj0.cyou")

	if !strings.Contains(out, `<title>AIOJ — Online Judge</title>`) {
		t.Errorf("site card should keep the marketing title:\n%s", out)
	}
	// Even the generic card needs the image: without one the platform renders
	// a solid colour block.
	if !strings.Contains(out, `<meta property="og:image" content="https://aioj0.cyou/og-default.png" />`) {
		t.Errorf("site card is missing og:image:\n%s", out)
	}
}

// An unconfigured PUBLIC_ORIGIN must not emit a relative or half-built URL:
// platforms require absolute ones, so the tags are omitted instead.
func TestRenderMetaPageOmitsAbsoluteTagsWithoutOrigin(t *testing.T) {
	out := renderCard(t, &ShareCard{Title: "Hello", Path: "/problems/hello"}, "")

	if strings.Contains(out, `rel="canonical"`) {
		t.Errorf("canonical emitted without an origin:\n%s", out)
	}
	if strings.Contains(out, "og:image") {
		t.Errorf("og:image emitted without an origin:\n%s", out)
	}
	// The entity title must survive regardless.
	if !strings.Contains(out, `<meta property="og:title" content="Hello" />`) {
		t.Errorf("og:title missing without an origin:\n%s", out)
	}
}

func TestPlainText(t *testing.T) {
	cases := map[string]string{
		"Print **hello** to stdout.":                      "Print hello to stdout.",
		"# Title\n\nSome `code` here.":                    "Title Some code here.",
		"See [the docs](https://example.com) for more.":   "See the docs for more.",
		"![diagram](https://example.com/x.png) Only text": "Only text",
		"Bounds: $1 \\le n \\le 10^5$ hold.":              "Bounds: hold.",
		"```\nfmt.Println()\n```":                         "",
		"Line one\nline two":                              "Line one line two",
	}
	for in, want := range cases {
		if got := PlainText(in, 200); got != want {
			t.Errorf("PlainText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlainTextTruncatesOnRuneBoundary(t *testing.T) {
	// Ten multi-byte characters; cutting by byte would split one in half and
	// produce invalid UTF-8 in the meta tag.
	in := strings.Repeat("é", 100)
	got := PlainText(in, 11)
	if len([]rune(got)) != 11 {
		t.Fatalf("got %d runes, want 11: %q", len([]rune(got)), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated text should end with an ellipsis: %q", got)
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Errorf("truncation split a rune: %q", got)
	}
}

func newMetaTestRouter(h *MetaHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/meta", h.Page)
	r.Get("/meta/*", h.Page)
	r.Get("/meta/problems/{slug}", h.Problem)
	return r
}

func TestMetaHandlerProblem(t *testing.T) {
	h := &MetaHandler{
		Origin: func() string { return "https://aioj0.cyou" },
		ProblemCard: func(_ context.Context, slug string) (*ShareCard, error) {
			if slug != "hello" {
				return nil, nil
			}
			return &ShareCard{Title: "Hello", Description: "Print hello.", Path: "/problems/" + slug, Kind: "article"}, nil
		},
	}
	srv := newMetaTestRouter(h)

	t.Run("public problem", func(t *testing.T) {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/problems/hello", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<title>Hello — AIOJ</title>`) {
			t.Errorf("response is missing the problem title:\n%s", body)
		}
	})

	// Unknown or private slugs must 404: serving a generic 200 for every
	// mistyped slug would make them all indexable duplicates of the home page.
	t.Run("unknown or private problem", func(t *testing.T) {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/problems/nope", nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "nope — AIOJ") {
			t.Errorf("the slug leaked into the title:\n%s", rec.Body.String())
		}
	})

	// A failed lookup must still unfurl as a link: the alternative is a 500 the
	// platform renders as a dead link.
	t.Run("lookup error degrades to the site card", func(t *testing.T) {
		broken := &MetaHandler{
			Origin: func() string { return "https://aioj0.cyou" },
			ProblemCard: func(context.Context, string) (*ShareCard, error) {
				return nil, errors.New("db down")
			},
		}
		rec := httptest.NewRecorder()
		newMetaTestRouter(broken).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/problems/hello", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "og:image") {
			t.Errorf("degraded card should still carry an image:\n%s", rec.Body.String())
		}
	})
}

// Every other app route gets the generic card, but still with the page's own
// canonical URL and an image.
func TestMetaHandlerPageStripsPrefix(t *testing.T) {
	h := &MetaHandler{Origin: func() string { return "https://aioj0.cyou" }}
	srv := newMetaTestRouter(h)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meta/contests", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `rel="canonical" href="https://aioj0.cyou/contests"`) {
		t.Errorf("canonical should point at the app route, not /meta:\n%s", body)
	}
	if !strings.Contains(body, `<title>AIOJ — Online Judge</title>`) {
		t.Errorf("list route should get the site card:\n%s", body)
	}
}
