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

// ValidateWiring reports every required handler that was left nil.
//
// Returns an empty slice when the wiring is complete. Callers should treat a
// non-empty result as fatal: serving with a gap means a nil dereference per
// request, and the operator needs to know at boot rather than from a user report.
//
// The optional fields are not checked, because they are not in the struct being
// checked: Deps.Optional is a separate type, so "may be unset" is expressed by
// where a field lives rather than by a list that has to be maintained beside it.
// That is the whole point of the split — the previous version kept a hand-written
// map of optional names, and TestOptionalListMatchesMountGuards existed only to
// keep that map honest.
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
		if !field.IsExported() || field.Name == "Optional" {
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
		if v.Field(i).IsNil() {
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
			"Wire each in main.go, or move it to Deps.Optional if the route is "+
			"deliberately mounted only when configured.",
		len(missing), strings.Join(missing, ", "))
}
