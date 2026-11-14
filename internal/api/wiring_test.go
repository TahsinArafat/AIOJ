package api

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
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
			// An interface needs a concrete value implementing it, and reflection
			// cannot invent one — calling Elem on an interface type panics, which
			// is what the first version of this fixture did. APIKeys and
			// APIKeyUsers are the only interface fields, and both are set below.
		}
	}

	d.Optional.APIKeys = stubAPIKeyVerifier{}
	d.Optional.APIKeyUsers = stubAPIKeyUserLookup{}

	return d
}

// stubAPIKeyVerifier satisfies APIVerifier for the fixture.
type stubAPIKeyVerifier struct{}

func (stubAPIKeyVerifier) GetByHash(context.Context, string) (*model.APIKey, error) {
	return &model.APIKey{}, nil
}
func (stubAPIKeyVerifier) UpdateLastUsed(context.Context, string) error { return nil }
func (stubAPIKeyVerifier) IncrementRequestCount(context.Context, string, time.Time) error {
	return nil
}
func (stubAPIKeyVerifier) GetRequestCount(context.Context, string, time.Time) (int, error) {
	return 0, nil
}

// stubAPIKeyUserLookup satisfies APIUserLookup for the fixture.
type stubAPIKeyUserLookup struct{}

func (stubAPIKeyUserLookup) GetByID(context.Context, string) (*model.User, error) {
	return &model.User{}, nil
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
	d.Optional.Meta = nil
	d.Optional.Health = nil
	d.Optional.Media = nil

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

// The required and optional sets are now types, not lists, so the property to
// pin is structural: every nil-guarded field lives in Deps.Optional, and no field
// in Deps (outside Optional) is guarded.
//
// This replaces a test that checked a hand-written map against the guards. The map
// is gone, so there is nothing to keep in sync — but the *placement* can still be
// wrong, and this is what says so.
func TestOptionalFieldsLiveInOptional(t *testing.T) {
	guarded := map[string]bool{}

	files, err := filepath.Glob("router*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("found no router files; the test cannot see the mounts")
	}

	// Matches both `b.Optional.X != nil` and a bare `b.X != nil`.
	guard := regexp.MustCompile(`if\s+b\.(?:Optional\.)?([A-Z][A-Za-z]*)\s*!=\s*nil`)
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

	if len(guarded) == 0 {
		t.Fatal("found no nil guards; the test cannot see the mounts")
	}

	// Every guarded field must be declared on OptionalDeps. A guarded field left
	// on Deps means the guard is dead code and the field is really required —
	// ValidateWiring would refuse to boot without it while the mount file says it
	// may be absent.
	optType := reflect.TypeOf(OptionalDeps{})
	for name := range guarded {
		if _, ok := optType.FieldByName(name); !ok {
			t.Errorf("%s is nil-guarded in a mount file but is not a field of "+
				"OptionalDeps, so the guard is unreachable", name)
		}
	}

	// And nothing in OptionalDeps may be guarded at the top level.
	depsType := reflect.TypeOf(Deps{})
	optField, _ := depsType.FieldByName("Optional")
	for i := 0; i < optType.NumField(); i++ {
		name := optType.Field(i).Name
		if _, ok := depsType.FieldByName(name); ok {
			t.Errorf("%s is declared on both Deps and OptionalDeps", name)
		}
	}
	_ = optField
}
