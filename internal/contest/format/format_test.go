package format_test

import (
	"encoding/json"
	"testing"

	"github.com/tahsinarafat/aioj/internal/contest/format"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/acm"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/atcoder"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/codeforces"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/ioi"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/oi"
)

func TestRegistry(t *testing.T) {
	list := format.List()
	expected := map[string]bool{
		"acm":        true,
		"oi":         true,
		"ioi":        true,
		"atcoder":    true,
		"codeforces": true,
	}

	for _, name := range list {
		delete(expected, name)
	}

	if len(expected) > 0 {
		t.Errorf("missing expected formats in registry: %v", expected)
	}

	_, ok := format.Get("acm")
	if !ok {
		t.Error("expected acm factory to be registered")
	}

	_, ok = format.Get("unknown")
	if ok {
		t.Error("expected unknown factory to not be registered")
	}

	cf, err := format.Create("acm", json.RawMessage(`{"penalty_per_wrong": 20}`))
	if err != nil {
		t.Fatalf("unexpected error creating acm format: %v", err)
	}
	if cf.Name() != "acm" {
		t.Errorf("expected format name 'acm', got %q", cf.Name())
	}
}
