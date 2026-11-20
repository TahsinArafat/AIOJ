package api

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

/**
 * docs/openapi/openapi.yaml claims to describe the API. It described five
 * paths out of 354, and nothing checked — so the document drifted silently and
 * the API Reference page in the app rendered a five-endpoint API for a
 * platform with hundreds.
 *
 * This pins the gap so it can only shrink deliberately. It is a ratchet, not an
 * equality check: demanding the spec match the router today would fail the
 * build on day one and be deleted or blanket-skipped within a week (the fate of
 * every check that lands red). Instead the number of undocumented routes may
 * only go DOWN. Documenting a route lowers the bound; adding an undocumented
 * one raises it and fails here, which is the moment to decide whether it should
 * be documented.
 *
 * Regenerate the bound after documenting routes:
 *
 *   UPDATE_OPENAPI_COVERAGE=1 go test ./internal/api/ -run TestOpenAPICoversTheRouter
 */

// undocumentedRouteCeiling is the largest number of mounted paths that may be
// absent from the OpenAPI document. Measured, not guessed: 234 paths are
// mounted and the spec documents 5, so the honest starting point is 230. Lower
// it as the spec grows; it must never rise.
const undocumentedRouteCeiling = 230

// openAPIPathRe matches a path key in the YAML document ("  /api/health:").
var openAPIPathRe = regexp.MustCompile(`(?m)^\s{2}(/\S*):\s*$`)

// openAPIDocumentedPaths reads the spec's paths. It is a line scan rather than a
// YAML parse on purpose: the only structure this test needs is "which path keys
// are present", and a scan keeps the test independent of the spec's formatting
// and of any YAML dependency.
func openAPIDocumentedPaths(t *testing.T) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile("../../docs/openapi/openapi.yaml")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	// Only the paths: section counts; a path-like pair elsewhere is prose.
	text := string(raw)
	idx := strings.Index(text, "\npaths:")
	if idx < 0 {
		t.Fatal("the spec has no paths: section")
	}
	section := text[idx:]
	if end := strings.Index(section, "\ncomponents:"); end > 0 {
		section = section[:end]
	}

	paths := map[string]bool{}
	for _, m := range openAPIPathRe.FindAllStringSubmatch(section, -1) {
		paths[m[1]] = true
	}
	return paths
}

// TestOpenAPICoversTheRouter counts the routes the document does not describe and
// fails when that count grows.
func TestOpenAPICoversTheRouter(t *testing.T) {
	t.Setenv("RATE_LIMIT_BURST", "1000000")
	t.Setenv("AUTH_RATE_LIMIT_BURST", "1000000")

	jwt := newTestJWT(t)
	mux := NewRouter(testDeps(), jwt)
	routes := collectRoutes(t, mux, jwt)
	if len(routes) < 50 {
		t.Fatalf("only %d routes mounted; the walk is broken", len(routes))
	}

	// The walk yields one entry per method+path. Compare per path: the spec
	// documents a path, and per-method documentation is a separate (larger)
	// piece of work this ratchet is not the right tool for.
	mounted := map[string]bool{}
	for _, l := range routes {
		mounted[l.Path] = true
	}

	documented := openAPIDocumentedPaths(t)
	missing := make([]string, 0, len(mounted))
	for path := range mounted {
		if !documented[path] {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)

	t.Logf("%d paths mounted, %d documented, %d undocumented",
		len(mounted), len(documented), len(missing))

	if os.Getenv("UPDATE_OPENAPI_COVERAGE") != "" {
		t.Logf("undocumentedRouteCeiling = %d", len(missing))
		return
	}

	if len(missing) > undocumentedRouteCeiling {
		shown := missing
		if len(shown) > 20 {
			shown = shown[:20]
		}
		t.Fatalf("%d paths are missing from docs/openapi/openapi.yaml, ceiling is %d.\n"+
			"Document them, or raise undocumentedRouteCeiling in this file if the growth is intended.\n"+
			"First undocumented paths:\n  %s",
			len(missing), undocumentedRouteCeiling, strings.Join(shown, "\n  "))
	}
}

// TestOpenAPISpecIsNotEmpty keeps the ratchet honest. If the spec were emptied,
// the test above would fail on the count — but a spec whose paths: section
// vanished entirely would make the reader "zero documented paths" look like a
// legitimate state rather than a deletion.
func TestOpenAPISpecIsNotEmpty(t *testing.T) {
	documented := openAPIDocumentedPaths(t)
	if len(documented) == 0 {
		t.Fatal("the OpenAPI document has no paths; it was emptied rather than edited")
	}
}
