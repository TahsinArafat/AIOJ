package handler

import (
	"bytes"
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Crawler-facing HTML for link previews.
//
// The app is a client-rendered SPA: every route is served `index.html`, and the
// page title is set from JavaScript (`web/src/lib/documentTitle.ts`). Browsers
// render that; the crawlers that build link previews do not — Telegram, Slack,
// Discord, WhatsApp, X, LinkedIn and Facebook fetch the HTML and read `<head>`.
// So sharing https://aioj0.cyou/problems/hello produced a card that said
// "AIOJ — Online Judge" with no image: the problem's own title only ever
// existed in the DOM after hydration.
//
// nginx sends social-network user agents for app routes here (`/meta/<path>`),
// this handler renders the same page as a small standalone document with the
// entity's title, description, canonical URL and Open Graph tags. Real browsers
// keep getting the SPA, so nothing about the app changes; the document below is
// metadata plus a readable fallback body, which is exactly what a preview
// crawler consumes.

const (
	siteName = "AIOJ"
	// siteTagline mirrors HOME_TITLE in web/src/lib/documentTitle.ts: the home
	// page and any page without an entity keeps the fuller marketing title.
	siteTagline = siteName + " — Online Judge"
	// siteDescription is the fallback summary for pages with no entity.
	siteDescription = "A competitive programming judge: solve problems, join contests and track your rating."
	// defaultCardImage is a 1200x630 PNG in web/public, served from the site
	// root. Platforms reject a card without an image and fall back to a solid
	// colour block, which is what a bare link used to look like.
	defaultCardImage = "/og-default.png"
	// descriptionLimit is roughly what previews show before truncating; longer
	// text is wasted bytes every crawler has to fetch.
	descriptionLimit = 200
)

// ShareCard is the data a link-preview crawler needs for one page.
type ShareCard struct {
	// Title is the entity name, e.g. a problem title. Empty means "no entity".
	Title string
	// Description is plain text; markdown is stripped by the caller with
	// PlainText, since previews do not render it.
	Description string
	// Path is the canonical in-app path, e.g. "/problems/hello".
	Path string
	// Kind maps to og:type. Defaults to "website".
	Kind string
	// Facts are short labels rendered in the fallback body, e.g. "Easy",
	// "2.0 s", "256 MB".
	Facts []string
}

// MetaHandler renders the crawler-facing copy of an app route.
type MetaHandler struct {
	// ProblemCard resolves a problem slug to its share card. It returns
	// (nil, nil) when the slug is unknown or the problem is not publicly
	// visible — the caller must answer 404 rather than publish the title of a
	// private problem to a crawler.
	ProblemCard func(ctx context.Context, slug string) (*ShareCard, error)
	// Origin returns the absolute site origin (e.g. "https://aioj0.cyou"), or
	// "" when unconfigured. Used for canonical and og:image URLs, which
	// platforms require to be absolute.
	Origin func() string
}

// pageTemplate is the standalone document served to crawlers. Kept minimal on
// purpose: no app bundle, no scripts. `html/template` escapes every value in
// attribute context, so a problem title containing quotes or angle brackets
// cannot break out of the tag.
var pageTemplate = template.Must(template.New("meta").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}" />
{{- if .Canonical}}
<link rel="canonical" href="{{.Canonical}}" />
{{- end}}
<meta property="og:type" content="{{.Kind}}" />
<meta property="og:site_name" content="{{.SiteName}}" />
<meta property="og:title" content="{{.OGTitle}}" />
<meta property="og:description" content="{{.Description}}" />
{{- if .Canonical}}
<meta property="og:url" content="{{.Canonical}}" />
{{- end}}
{{- if .Image}}
<meta property="og:image" content="{{.Image}}" />
<meta property="og:image:type" content="image/png" />
<meta property="og:image:width" content="1200" />
<meta property="og:image:height" content="630" />
<meta property="og:image:alt" content="{{.SiteName}} — Online Judge" />
{{- end}}
<meta name="twitter:card" content="summary_large_image" />
<meta name="twitter:title" content="{{.OGTitle}}" />
<meta name="twitter:description" content="{{.Description}}" />
{{- if .Image}}
<meta name="twitter:image" content="{{.Image}}" />
{{- end}}
<meta name="theme-color" content="#2563eb" />
<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
</head>
<body>
<main>
<h1>{{.Heading}}</h1>
<p>{{.Description}}</p>
{{- if .Facts}}
<ul>
{{- range .Facts}}
<li>{{.}}</li>
{{- end}}
</ul>
{{- end}}
<p><a href="{{.Link}}">Open on {{.SiteName}}</a></p>
</main>
</body>
</html>
`))

type metaPage struct {
	SiteName    string
	Title       string
	OGTitle     string
	Heading     string
	Description string
	Canonical   string
	Link        string
	Image       string
	Kind        string
	Facts       []string
}

// RenderMetaPage builds the crawler-facing document for one card.
//
// Exported so the HTML shape can be asserted in tests without a database, the
// same reason BuildSitemapXML is a pure function.
func RenderMetaPage(card *ShareCard, origin string) ([]byte, error) {
	if card == nil {
		card = siteCard("/")
	}

	path := strings.TrimSpace(card.Path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	title := strings.TrimSpace(card.Title)
	if title == "" || title == siteName {
		title = siteTagline
	}

	description := strings.TrimSpace(card.Description)
	if description == "" {
		description = siteDescription
	}
	// Lead the preview with the facts a solver scans for (difficulty, limits)
	// instead of the statement's opening sentence, which is usually boilerplate.
	if len(card.Facts) > 0 {
		description = strings.Join(card.Facts, " · ") + " — " + description
	}

	kind := strings.TrimSpace(card.Kind)
	if kind == "" {
		kind = "website"
	}

	base := NormalizeOrigin(origin)
	page := metaPage{
		SiteName:    siteName,
		Title:       title,
		OGTitle:     title,
		Heading:     title,
		Description: PlainText(description, descriptionLimit),
		Link:        base + path,
		Kind:        kind,
		Facts:       card.Facts,
	}
	// For a real entity `<title>` keeps the tab-title convention a browser tab
	// uses ("Hello — AIOJ"), while og:title stays the bare name: every card is
	// already labelled with the site name and origin, so "Hello" reads better
	// there than "Hello — AIOJ".
	if title != siteTagline {
		page.Title = title + " — " + siteName
		page.OGTitle = title
	}
	if base != "" {
		page.Canonical = base + path
		page.Image = base + defaultCardImage
	}

	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, page); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Problem serves the share card for one problem slug.
func (h *MetaHandler) Problem(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var card *ShareCard
	var err error
	if h.ProblemCard != nil {
		card, err = h.ProblemCard(r.Context(), slug)
	}
	if err != nil {
		// A link preview without the problem title still beats one with no
		// page at all, so degrade to the site card instead of failing the
		// request. The short max-age below means the next share retries.
		slog.Error("meta: problem card lookup failed", "slug", slug, "error", err)
		h.write(w, siteCard("/problems/"+slug), http.StatusOK, 60)
		return
	}
	if card == nil {
		// Unknown or non-public problem. 404 is deliberate: answering 200
		// with a generic card would make every mistyped slug an indexable
		// page that duplicates the site root.
		h.write(w, siteCard("/problems/"+slug), http.StatusNotFound, 60)
		return
	}
	h.write(w, card, http.StatusOK, 600)
}

// Page serves the share card for any other app route. It exists so a crawl of
// /contests, /blog or any future route gets a complete card (title, image,
// canonical) instead of the SPA shell with no Open Graph tags at all.
func (h *MetaHandler) Page(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/meta")
	if path == "" {
		path = "/"
	}
	h.write(w, siteCard(path), http.StatusOK, 600)
}

func (h *MetaHandler) write(w http.ResponseWriter, card *ShareCard, status, maxAge int) {
	origin := ""
	if h.Origin != nil {
		origin = h.Origin()
	}
	body, err := RenderMetaPage(card, origin)
	if err != nil {
		slog.Error("meta: render failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(maxAge))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// siteCard is the generic card for a route with no entity-specific metadata.
func siteCard(path string) *ShareCard {
	return &ShareCard{
		Title:       siteTagline,
		Description: siteDescription,
		Path:        path,
		Kind:        "website",
	}
}

var (
	fencedCodeRe = regexp.MustCompile("(?s)```.*?```")
	// Images carry no text a preview can show; links keep their label.
	imageRe   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkRe    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	htmlTagRe = regexp.MustCompile(`<[^>]*>`)
	// Statements are full of LaTeX ($n \le 10^5$). The source is noise in a
	// preview, and rendering it is out of scope here.
	mathRe = regexp.MustCompile(`\$\$?[^$]*\$\$?`)
	// Inline markdown emphasis and heading markers: dropped rather than
	// converted, because previews show one plain paragraph.
	markdownMarks = strings.NewReplacer("#", "", "*", "", "_", "", "`", "", ">", "", "|", "", "~", "")
)

// PlainText reduces a markdown problem statement to a single plain-text line
// suitable for a link preview, truncated to limit runes.
//
// Deliberately lossy and dependency-free: this runs on the crawler path, so it
// must not pull the markdown/LaTeX toolchain into the request, and the only
// consumer is a description meta tag.
func PlainText(markdown string, limit int) string {
	s := markdown
	s = fencedCodeRe.ReplaceAllString(s, " ")
	s = imageRe.ReplaceAllString(s, " ")
	s = linkRe.ReplaceAllString(s, "$1")
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = mathRe.ReplaceAllString(s, " ")
	s = markdownMarks.Replace(s)
	// Collapse every run of whitespace: statements are multi-line and a meta
	// description is a single line.
	s = strings.Join(strings.Fields(s), " ")
	return truncateRunes(s, limit)
}

// truncateRunes cuts s to at most limit runes, appending an ellipsis when it
// had to cut. Cutting on a rune boundary keeps multi-byte characters whole.
func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	// Leave room for the ellipsis so the result never exceeds the limit.
	return strings.TrimRight(string(runes[:limit-1]), " ") + "…"
}
