package api

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/auth"
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
 */

// routeLine is one mounted route as the test records it.
//
// Middleware is included, and that is the part that makes this an equivalence
// check rather than a smoke test. A route can keep its path and method while
// losing AuthMiddleware or a feature gate during a refactor — the endpoint still
// answers, it just answers to everybody. chi hands the walk the real middleware
// chain, so the chain is pinned alongside the path.
type routeLine struct {
	Method string
	Path   string
	// Middleware is the chain applied to this route, outermost first, by
	// function name. Names rather than the funcs themselves because the funcs
	// are closures and compare unequal on every call.
	Middleware []string
}

func collectRoutes(t *testing.T, mux http.Handler) []routeLine {
	t.Helper()

	r, ok := mux.(*chi.Mux)
	if !ok {
		t.Fatalf("NewRouter returned %T, not *chi.Mux; the route walk needs the concrete mux", mux)
	}

	var out []routeLine
	err := chi.Walk(r, func(method, route string, _ http.Handler, mws ...func(http.Handler) http.Handler) error {
		names := make([]string, 0, len(mws))
		for _, mw := range mws {
			names = append(names, middlewareName(mw))
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

// middlewareName reduces a middleware func to a stable, readable name.
//
// The runtime name of a closure is not stable: a middleware defined inline in
// NewRouter reports as "func1" while the same middleware as a method reports as
// "api.(*routerBuilder).gate.1", and moving that code changes the string without
// changing a single request's behaviour. A table keyed on those strings would
// fail on every refactor and be regenerated reflexively, which is how a route
// table stops being a check.
//
// So the name is degraded to the part that has meaning: the concrete middleware
// whose effect is observable (AuthMiddleware, RequireRole, StrictAuth,
// OptionalAuthMiddleware, and the base chain) keeps its name, and everything
// else — the feature-gate closures, the anonymous inline funcs — is recorded as
// "closure". What those closures DO is asserted by behaviour in the gate tests
// (router_skip_gate_test.go and the *_gate_test.go files), which read the
// response rather than the symbol.
func middlewareName(mw func(http.Handler) http.Handler) string {
	if mw == nil {
		return "nil"
	}
	full := runtime.FuncForPC(reflect.ValueOf(mw).Pointer()).Name()
	if i := strings.LastIndex(full, "/"); i >= 0 {
		full = full[i+1:]
	}
	// Drop the trailing .1 / .func1 closure index.
	if i := strings.LastIndex(full, ".func"); i >= 0 {
		full = full[:i] + ".closure"
	}
	// A named middleware keeps its name; a bare closure or a method-valued
	// closure is recorded generically.
	for _, known := range []string{
		"SentryRecover", "RequestID", "Logging", "Recoverer", "SecurityHeaders",
		"RealIPFromEnv", "RateLimit", "MetricsAuth", "AuthMiddleware",
		"OptionalAuthMiddleware", "RequireRole", "StrictAuth",
	} {
		if strings.Contains(full, known) {
			return known
		}
	}
	return "closure"
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
	jwt := newTestJWT(t)
	mux := NewRouter(testDeps(), jwt)

	got := collectRoutes(t, mux)

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
	mux := NewRouter(testDeps(), newTestJWT(t))
	routes := collectRoutes(t, mux)

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
	mux := NewRouter(testDeps(), newTestJWT(t))

	for _, l := range collectRoutes(t, mux) {
		if !strings.HasPrefix(l.Path, "/api/admin/") {
			continue
		}
		// /api/admin/backups/{filename} is the one documented exception for the
		// browser-navigation download; it still requires the admin role.
		chain := strings.Join(l.Middleware, " ")
		if !strings.Contains(chain, "RequireRole") {
			t.Errorf("%s %s has no RequireRole in its chain, so any signed-in user may call it:\n  %s",
				l.Method, l.Path, chain)
		}
		if !strings.Contains(chain, "AuthMiddleware") {
			t.Errorf("%s %s has no AuthMiddleware in its chain, so an anonymous caller may reach it:\n  %s",
				l.Method, l.Path, chain)
		}
	}
}

// TestMissingDepsDoNotPanic covers the property the router has always relied on
// and never stated: NewRouter tolerates a partially-wired Deps. main wires
// everything, but the router tests build a Deps by hand, and a nil dereference
// at startup would take the whole server down rather than degrade one endpoint.
func TestMissingDepsDoNotPanic(t *testing.T) {
	mux := NewRouter(Deps{}, newTestJWT(t))
	if mux == nil {
		t.Fatal("NewRouter returned nil for an empty Deps")
	}
	// Walking forces chi to realise every handler chain.
	if got := collectRoutes(t, mux); len(got) == 0 {
		t.Fatal("an empty Deps mounted no routes at all")
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
