package api

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

/**
 * Startup wiring validation.
 *
 * A handler the router mounts but main forgot to construct is a nil pointer, and
 * the first request to that route panics on it. The panic is recovered into an
 * empty 500, so the symptom is "this endpoint has never worked" with nothing
 * pointing at the cause.
 *
 * The type system cannot catch it — a nil pointer is a valid value for a pointer
// field, so ValidateWiring checks it at boot. These tests pin both halves: that
 * the check names a gap, and that the optional list still matches the guards in
 * the mount files, which is the part that would otherwise rot.
*/

// completeDeps fills every field ValidateWiring checks, so a test can start from
// a valid wiring and remove exactly one thing.
//
// testDeps() is not it: that fixture exists for the route-table tests and holds
// only the always-registered handlers, which is the opposite of complete.
func completeDeps(t *testing.T) Deps {
	t.Helper()
	d := Deps{}
	v := reflect.ValueOf(&d).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.Ptr:
			f.Set(reflect.New(f.Type().Elem()))
		case reflect.Func:
			f.Set(reflect.MakeFunc(f.Type(), func([]reflect.Value) []reflect.Value { return nil }))
		case reflect.Interface:
			// A non-nil interface needs a concrete value implementing it, and there
			// is no generic one. APIKeyUsers is the only interface field, and its
			// single method shape is known, so a stub satisfies it.
			stub := reflect.New(f.Type().Elem())
			if m, ok := f.Type().MethodByName(""); ok {
				_ = m
			}
			f.Set(stub)
		}
	}
	return d
}

func TestValidateWiringAcceptsCompleteDeps(t *testing.T) {
	if missing := ValidateWiring(completeDeps(t)); len(missing) != 0 {
		t.Errorf("a fully-populated Deps reported %v as missing", missing)
	}
}

// A gap must be named, and only the gap. An empty Deps has 51 unwired
// handlers, so the test asserts a specific one is reported rather than counting.
func TestValidateWiringNamesTheGap(t *testing.T) {
	d := completeDeps(t)
	d.Gym = nil

	missing := ValidateWiring(d)
	if len(missing) != 1 {
		t.Fatalf("got %d missing, want exactly 1 (Gym): %v", len(missing), missing)
	}
	if missing[0] != "Gym" {
		t.Errorf("reported %q, want %q", missing[0], "Gym")
	}
}

// An optional field left nil must not be reported, or every deployment that
// leaves a gated feature off would refuse to boot.
func TestValidateWiringIgnoresOptional(t *testing.T) {
	d := completeDeps(t)
	d.Meta = nil
	d.Health = nil
	d.Media = nil

	if missing := ValidateWiring(d); len(missing) != 0 {
		t.Errorf("optional fields were reported as missing: %v", missing)
	}
}

// A string or bool is not "unwired" — an empty metrics token is a real
// configuration (only admins may scrape), not a gap.
func TestValidateWiringIgnoresScalars(t *testing.T) {
	d := completeDeps(t)
	d.MetricsToken = ""
	d.AIEnabled = false
	d.CSRFSecret = ""

	if missing := ValidateWiring(d); len(missing) != 0 {
		t.Errorf("scalar fields were reported as missing: %v", missing)
	}
}

// The optional list is maintained by hand, so it can drift from the guards in
// the mount files. A field guarded in a mount but missing from Optional would
// make a legitimate deployment refuse to boot; an entry no longer guarded would
// let a required handler through. Both are checked against the source.
func TestOptionalListMatchesMountGuards(t *testing.T) {
	guarded := map[string]bool{}

	files, err := filepath.Glob("router*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("found no router files; the test cannot see the mounts")
	}

	guard := regexp.MustCompile(`if\s+(?:b\.|d\.)?([A-Z][A-Za-z]*)\s*!=\s*nil`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range guard.FindAllStringSubmatch(string(body), -1) {
			guarded[m[1]] = true
		}
	}

	for name := range guarded {
		if !Optional[name] {
			t.Errorf("%s is nil-guarded in a mount file but missing from Optional, "+
				"so a deployment that leaves it unset would refuse to boot", name)
		}
	}
	for name := range Optional {
		if !guarded[name] {
			t.Errorf("Optional lists %s but no mount file guards it, so a nil %s "+
				"would be reachable and panic on the first request", name, name)
		}
	}
}
