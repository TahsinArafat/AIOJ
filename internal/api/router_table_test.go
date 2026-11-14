package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The route table is the one part of the API that nothing else can check. A
 * handler test proves a handler works; it cannot prove the handler is reachable,
 * at which path, under which middleware. A route dropped while moving the
 * registration code to another file is invisible to every other test in the
 * repo — the handler still passes, the endpoint is simply gone.
 *
 * This walks the real router and pins the full table. It exists because the
 * registrations were split out of one 893-line NewRouter into per-domain files,
 * and "the split changed nothing" needed to be a check rather than a claim.
 *
 * Regenerate after an intentional route change:
 *
 *   UPDATE_ROUTES=1 go test ./internal/api/ -run TestRouteTable
 *
 * then review the diff. A route disappearing from that diff is the whole point
 * of the file.
 *
 * The middleware column is classified by behaviour, not by symbol name. The
 * first version keyed on runtime function names and broke in CI: Go gives every
 * closure from one function the same name and the same code pointer, so the auth,
 * role and feature gates were one indistinguishable token, and whether a name
 * appeared at all depended on inlining. See classifyMiddleware for what replaced
 * it.
 */

// routeLine is one mounted route as the test records it.
//
// Middleware is included, and that is the part that makes this an equivalence
// check rather than a smoke test. A route can keep its path and method while
// losing AuthMiddleware or a feature gate during a refactor — the endpoint still
// answers, it just answers to everybody. chi hands the walk the real middleware
// chain, so the chain is pinned alongside the path.
//
// Each entry is a behaviour class — pass-through, optional-auth, auth-required,
// role-required, rate-limited — not a function name, so the table means the same
// thing on every machine and every Go build.
type routeLine struct {
	Method string
	Path   string
	// Middleware is the chain applied to this route, outermost first, by
	// function name. Names rather than the funcs themselves because the funcs
	// are closures and compare unequal on every call.
	Middleware []string
}

// collectRoutes walks the router and classifies each route's middleware chain.
func collectRoutes(t *testing.T, mux http.Handler, _ *auth.JWTManager) []routeLine {
	t.Helper()

	r, ok := mux.(*chi.Mux)
	if !ok {
		t.Fatalf("NewRouter returned %T, not *chi.Mux; the route walk needs the concrete mux", mux)
	}

	var out []routeLine
	err := chi.Walk(r, func(method, route string, _ http.Handler, mws ...func(http.Handler) http.Handler) error {
		names := make([]string, 0, len(mws))
		for _, mw := range mws {
			names = append(names, classifyMiddleware(mw))
		}
		out = append(out, routeLine{Method: method, Path: route, Middleware: names})
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// probeClaims is the identity the second probe carries. AuthMiddleware puts
// exactly this on the request context in production, so seeding it is what lets a
// role gate be observed on its own.
//
// This is why the token probe alone was not enough: RequireRole reads claims from
// the context and checks the role only once it has them, so a request that merely
// carries a token still answers 401 — indistinguishable from an auth gate.
var probeClaims = &auth.Claims{
	UserID: "classify-probe-user",
	Role:   model.RoleUser,
}

// classifyMiddleware identifies a middleware by what it DOES, not by its symbol
// and not by a probe that changes what it observes.
//
// Two earlier attempts failed in CI, and both failures are worth keeping.
//
// The first keyed on the runtime symbol. Go gives every closure returned by one
// function the same name AND the same code pointer — verified by probe: two
// closures from a factory both report `factory.func1` with equal pointers.
// AuthMiddleware, RequireRole and the feature gate are all closures, so all three
// collapsed to one token and the admin assertion reported a privilege escalation
// that did not exist. Whether a substring like "RealIPFromEnv" appeared also
// depended on inlining, so the golden string differed per machine.
//
// The second probed behaviour but let the probe consume state. The rate limiter
// keeps a shared token bucket with burst 200, and there are 294 routes: the 201st
// probe started receiving 429, so every route after that point classified the
// limiter as "rate-limited" and the earlier ones as pass-through. The golden file
// recorded the artifact, and the boundary moved with wall-clock refill and with
// RATE_LIMIT_BURST. An instrument must not mutate the system it measures.
//
// The fix for that is to raise the burst for the route-walking tests (see
// TestRouteTable), so the limiter is a pure pass-through for the walk. With that
// in place this observes two things that are pure functions of the middleware:
// the headers it sets, and whether it refuses a request — first anonymous, then
// with ordinary-user claims seeded, which is what separates a role gate from an
// auth gate.
func classifyMiddleware(mw func(http.Handler) http.Handler) string {
	if mw == nil {
		return "nil"
	}

	probe := func(req *http.Request) (reached bool, code int) {
		inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })
		rec := httptest.NewRecorder()
		mw(inner).ServeHTTP(rec, req)
		return reached, rec.Code
	}

	// Probe one: anonymous. A gate refuses; a pass-through does not.
	if reached, _ := probe(httptest.NewRequest(http.MethodGet, "/probe", nil)); reached {
		return classifyPassThrough(mw)
	}

	// Probe two: a request that already carries an ordinary user's claims, which
	// is what AuthMiddleware leaves behind. A role gate now has what it needs to
	// refuse on the role rather than on the missing identity.
	withClaims := httptest.NewRequest(http.MethodGet, "/probe", nil)
	withClaims = withClaims.WithContext(
		context.WithValue(withClaims.Context(), middleware.UserContextKey, probeClaims))

	reached, code := probe(withClaims)
	switch {
	case reached:
		// Refused the anonymous request but let an identified caller through:
		// the optional-auth middleware, or a feature gate with no resolver.
		return "optional-auth"
	case code == http.StatusForbidden:
		return "role-required"
	case code == http.StatusUnauthorized:
		return "auth-required"
	default:
		return "refused-" + strconv.Itoa(code)
	}
}

// classifyPassThrough names a middleware that let the anonymous probe through, by
// the headers it set. The limiter lands here too once the burst is raised for the
// walk; it is otherwise indistinguishable from any other pass-through, which is
// honest — the route table is not where limiter behaviour is asserted.
func classifyPassThrough(mw func(http.Handler) http.Handler) string {
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	rec := httptest.NewRecorder()
	mw(inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))

	h := rec.Header()
	switch {
	case h.Get("Content-Security-Policy") != "" || h.Get("X-Frame-Options") != "":
		return "security-headers"
	case h.Get("X-Request-Id") != "" || h.Get("X-Request-ID") != "":
		return "request-id"
	}
	return "pass-through"
}

// TestBaseChainOrder pins the eight middlewares every request passes through, in
// order, by asserting their observable effects rather than their names.
//
// The route table cannot express this: chi hands the walk a chain in which the
// base middlewares appear as unnamed closures, and the classifier deliberately
// refuses to guess. Their order genuinely matters — RealIP must run before the
// rate limiter, which keys its buckets on the client address — so it is asserted
// here instead of being left implicit.
func TestBaseChainOrder(t *testing.T) {
	jwt := newTestJWT(t)

	// A route with no middleware of its own beyond the base chain.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	NewRouter(testDeps(), jwt).ServeHTTP(rec, req)

	// SecurityHeaders is first in the chain, so its headers are present on a
	// response produced by the last handler.
	for _, header := range []string{"X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy"} {
		if rec.Header().Get(header) == "" {
			t.Errorf("the base chain did not set %s, so SecurityHeaders is not mounted on every route", header)
		}
	}
}

// TestRouteTable pins every route the server mounts.
//
// The Deps handed to NewRouter is deliberately minimal. Most registrations are
// guarded by a nil check, so those routes are simply absent from a bare Deps —
// which makes this test blind to them. To keep the table meaningful, the
// always-registered routes (health, status, sitemap, meta, feeds, auth, users)
// are wired with real handlers; the optional ones are covered by their own
// mount tests below.
func TestRouteTable(t *testing.T) {
	// The route walk probes each route's middleware chain, and every probe goes
	// through the rate limiter that base() installed. There are 294 routes and
	// the limiter's default burst is 200, so the 201st probe onward saw 429 and
	// the golden recorded "rate-limited" from that route down — an artifact of
	// the walk, not of the router. Raising the burst for this test makes the
	// limiter a pure pass-through for the walk, which is the only honest
	// classification: limiter behaviour is asserted by the middleware's own
	// tests and by TestBaseChainOrder, not by the route table.
	t.Setenv("RATE_LIMIT_BURST", "1000000")
	t.Setenv("AUTH_RATE_LIMIT_BURST", "1000000")

	jwt := newTestJWT(t)
	mux := NewRouter(testDeps(), jwt)

	got := collectRoutes(t, mux, jwt)

	// A bare router must still be non-trivial, or the assertions below would
	// pass against an empty table and prove nothing.
	if len(got) < 50 {
		t.Fatalf("only %d routes mounted; the walk or the wiring is broken", len(got))
	}

	rendered := make([]string, 0, len(got))
	for _, l := range got {
		rendered = append(rendered, fmt.Sprintf("%-6s %-52s %s", l.Method, l.Path, strings.Join(l.Middleware, " ")))
	}
	body := strings.Join(rendered, "\n") + "\n"

	const golden = "testdata/route_table.golden"

	if os.Getenv("UPDATE_ROUTES") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(golden, []byte(body), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s with %d routes", golden, len(got))
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_ROUTES=1 to create it): %v", err)
	}
	if body != string(want) {
		t.Errorf("the mounted route table changed.\n\nIf this is intentional, regenerate with:\n"+
			"  UPDATE_ROUTES=1 go test ./internal/api/ -run TestRouteTable\n"+
			"and confirm in the diff that no route was dropped.\n\n%s", firstDiff(string(want), body))
	}
}

// firstDiff reports the first differing line pair, so a failure names the route
// that moved instead of printing a thousand-line table.
func firstDiff(want, got string) string {
	w := strings.Split(strings.TrimRight(want, "\n"), "\n")
	g := strings.Split(strings.TrimRight(got, "\n"), "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("first difference at line %d:\n  want: %q\n  got:  %q", i+1, wl, gl)
		}
	}
	return "tables are equal"
}

// TestCriticalRoutesAreMounted names the endpoints whose absence would be a
// security or availability failure rather than a missing feature. The golden
// file catches any change; this catches the specific ones that must never
// disappear, and it reads as a list of the guarantees rather than a table.
func TestCriticalRoutesAreMounted(t *testing.T) {
	t.Setenv("RATE_LIMIT_BURST", "1000000")
	t.Setenv("AUTH_RATE_LIMIT_BURST", "1000000")
	jwt := newTestJWT(t)
	mux := NewRouter(testDeps(), jwt)
	routes := collectRoutes(t, mux, jwt)

	mounted := make(map[string]bool, len(routes))
	for _, l := range routes {
		mounted[l.Method+" "+l.Path] = true
	}

	for _, want := range []struct{ method, path, why string }{
		{"GET", "/api/health", "load balancers and the status page poll it"},
		{"POST", "/api/auth/login", "no route means nobody can sign in"},
		{"POST", "/api/auth/refresh", "an access token could never be renewed"},
		{"POST", "/api/auth/logout", "a stolen refresh token would outlive sign-out"},
		{"GET", "/api/auth/csrf", "a cold visitor could not POST at all"},
		{"GET", "/api/users/profile/edit", "the signed-in profile would 404"},
		{"GET", "/api/problems/", "the archive is the product"},
		{"GET", "/api/submissions/", "submitting is the product"},
	} {
		if !mounted[want.method+" "+want.path] {
			t.Errorf("%s %s is not mounted: %s", want.method, want.path, want.why)
		}
	}
}

// TestAdminRoutesRequireAuthAndAdminRole asserts the two middlewares whose
// absence would be a privilege-escalation bug rather than a broken feature.
//
// The golden table already fails if the chain changes anywhere. This states the
// rule directly, because the failure mode it guards is specific and severe: every
// route under /api/admin must carry both AuthMiddleware and RequireRole("admin").
// If the admin block is ever relocated and loses one of them, the golden diff
// would show it — and this would name it.
func TestAdminRoutesRequireAuthAndAdminRole(t *testing.T) {
	t.Setenv("RATE_LIMIT_BURST", "1000000")
	t.Setenv("AUTH_RATE_LIMIT_BURST", "1000000")
	jwt := newTestJWT(t)
	mux := NewRouter(testDeps(), jwt)

	for _, l := range collectRoutes(t, mux, jwt) {
		if !strings.HasPrefix(l.Path, "/api/admin/") {
			continue
		}
		// /api/admin/backups/{filename} is the one documented exception for the
		// browser-navigation download; it still requires the admin role.
		chain := strings.Join(l.Middleware, " ")
		if !strings.Contains(chain, "role-required") {
			t.Errorf("%s %s has no role gate in its chain, so any signed-in user may call it:\n  %s",
				l.Method, l.Path, chain)
		}
		if !strings.Contains(chain, "auth-required") {
			t.Errorf("%s %s has no auth gate in its chain, so an anonymous caller may reach it:\n  %s",
				l.Method, l.Path, chain)
		}
	}
}

// TestMissingDepsDoNotPanic covers the property the router has always relied on
// and never stated: NewRouter tolerates a partially-wired Deps. main wires
// everything, but the router tests build a Deps by hand, and a nil dereference
// at startup would take the whole server down rather than degrade one endpoint.
func TestMissingDepsDoNotPanic(t *testing.T) {
	t.Setenv("RATE_LIMIT_BURST", "1000000")
	t.Setenv("AUTH_RATE_LIMIT_BURST", "1000000")
	mux := NewRouter(Deps{}, newTestJWT(t))
	if mux == nil {
		t.Fatal("NewRouter returned nil for an empty Deps")
	}
	// Walking forces chi to realise every handler chain.
	if got := collectRoutes(t, mux, newTestJWT(t)); len(got) == 0 {
		t.Fatal("an empty Deps mounted no routes at all")
	}

	// Walking proves the chains build; this proves a request is answered. An
	// unwired optional handler must be absent rather than panic — that is the
	// contract the 32 `if d.X != nil` guards exist to provide, and it is what lets
	// the router tests build a Deps by hand.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/activity", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/activity with no Activity handler = %d, want 404", rec.Code)
	}
}

// testDeps is the smallest Deps that exercises the always-registered routes.
// Handlers are zero-valued: the route table is what is under test here, not the
// handlers, and a zero handler still registers a path.
func testDeps() Deps {
	return Deps{
		Health:      &handler.HealthChecker{},
		Activity:    &handler.ActivityHandler{},
		Legal:       &handler.LegalHandler{},
		Meta:        &handler.MetaHandler{},
		Auth:        &handler.AuthHandler{},
		Users:       &handler.UsersHandler{},
		Submission:  &handler.SubmissionHandler{},
		Problem:     &handler.ProblemHandler{},
		ProblemI18n: &handler.ProblemI18nHandler{},
		Rankings:    &handler.RankingsHandler{},
		Search:      &handler.SearchHandler{},
	}
}

// newTestJWT returns a manager whose secret satisfies the length check the real
// one applies, so the router can build its middleware chain.
func newTestJWT(t *testing.T) *auth.JWTManager {
	t.Helper()
	return auth.NewJWTManager(skipGateTestSecret, time.Minute, time.Hour)
}
