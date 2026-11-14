package api

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

/**
 * Startup validation for a wired Deps.
 *
 * A handler the router mounts but main forgot to construct is a nil pointer, and
 * the first request to that route panics on it. The panic is caught by the
 * recovery middleware and returned as an empty 500, so the symptom is "this one
 * endpoint has never worked" rather than anything that points at the cause —
 * which is exactly how a route can ship broken and stay broken.
 *
 * The type system cannot catch it, because the fields are pointers and a nil
 * pointer is a valid value. So this checks the wiring once, at startup, and
 * refuses to serve rather than discovering the gap one request at a time.
 *
 * It lives here rather than in NewRouter because the router is deliberately
 * permissive: the router tests build a Deps holding only the handler under test,
// and TestMissingDepsDoNotPanic asserts that a partially-wired Deps degrades to a
 * 404 rather than failing. main is the one caller that must be complete.
*/

// Optional lists the Deps fields a deployment may legitimately leave unset.
//
// Each is guarded by an `if d.X != nil` in its mount file, so an absent one
// means a route is not registered rather than a nil handler being reachable.
// Adding an optional field here is the only edit needed when a new gated feature
// gets a handler — the compiler cannot tell the two kinds apart, so this is the
// one list that has to be maintained by hand.
//
// Derived from the guards rather than guessed: a field is optional exactly when
// its mount file checks it for nil.
var Optional = map[string]bool{
	// Crawl and feed surfaces: served only when the operator configured a source.
	"Sitemap":     true,
	"Feed":        true,
	"BlogFeed":    true,
	"ContestFeed": true,
	"CommentFeed": true,
	"Meta":        true,
	"Legal":       true,
	// Feature-gated or dev-only surfaces.
	"Media":           true,
	"PublicLanguages": true,
	"CDN":             true,
	"Achievements":    true,
	"Friendships":     true,
	"AuditLog":        true,
	"Health":          true,
	"Activity":        true,
	"AIGeneration":    true,
	"DevMail":         true,
	"AuthMethods":     true,
	"APIKeys":         true,
	// Auth flows that are configured, not built in.
	"VerifyEmail":   true,
	"TwoFA":         true,
	"TwoFAVerify":   true,
	"OAuthStart":    true,
	"OAuthCallback": true,
	// Self-service account actions, mounted only when wired.
	"UsersExport":   true,
	"UsersDeletion": true,
}

// ValidateWiring reports every required handler that was left nil.
//
// Returns an empty slice when the wiring is complete. Callers should treat a
// non-empty result as fatal: serving with a gap means a nil dereference per
// request, and the operator needs to know at boot rather than from a user report.
//
// Reflection is used because the alternative is a hand-written list of 51 nil
// checks that would drift from Deps the moment a field is added. The field names
// in the error are the Go names, which is what the person editing main.go needs.
func ValidateWiring(d Deps) []string {
	v := reflect.ValueOf(d)
	t := v.Type()

	var missing []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		// Only pointer and func fields can be "not wired". A string or bool has a
		// meaningful zero value, and treating "" as unwired would reject a
		// deployment that legitimately leaves the metrics token empty.
		switch v.Field(i).Kind() {
		case reflect.Ptr, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice:
		default:
			continue
		}
		if v.Field(i).IsNil() && !Optional[field.Name] {
			missing = append(missing, field.Name)
		}
	}
	sort.Strings(missing)
	return missing
}

// ValidateWiringError is ValidateWiring as an error, for a caller that wants to
// fail with a message.
func ValidateWiringError(d Deps) error {
	missing := ValidateWiring(d)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"%d required handler(s) were not wired, so their routes would panic on the "+
			"first request: %s.\n"+
			"Wire each in main.go, or add it to api.Optional if the route is "+
			"deliberately mounted only when configured.",
		len(missing), strings.Join(missing, ", "))
}
