package compiler

import (
	"encoding/json"
	"testing"

	"github.com/tahsinarafat/aioj/lang"
	"gopkg.in/yaml.v3"
)

// The seeder reads this, so a regression here means a fresh install has no
// languages at all — an empty submit dropdown and every submission rejected as
// "unsupported language". It would also pass silently: Seed of an empty map is a
// successful no-op.
func TestShippedDefaultsCoversEveryShippedFile(t *testing.T) {
	got, err := ShippedDefaults()
	if err != nil {
		t.Fatalf("ShippedDefaults: %v", err)
	}

	keys := lang.Keys()
	if len(keys) == 0 {
		t.Fatal("lang.Keys() is empty; the embed directive is not matching")
	}
	if len(got) != len(keys) {
		t.Fatalf("ShippedDefaults returned %d configs, want %d (one per lang/*.yaml)", len(got), len(keys))
	}

	for _, key := range keys {
		doc, ok := got[key]
		if !ok {
			t.Errorf("no shipped default for %q", key)
			continue
		}
		var cfg LangConfig
		if err := json.Unmarshal(doc, &cfg); err != nil {
			t.Errorf("%s: stored document is not valid JSON: %v", key, err)
			continue
		}
		if cfg.Key == "" {
			t.Errorf("%s: seeded config has no key", key)
		}
		if cfg.Name == "" {
			t.Errorf("%s: seeded config has no name; the dropdown would render it blank", key)
		}
		// A zero multiplier renders as "0 s" on the problem page.
		if cfg.TimeLimitMultiplier <= 0 || cfg.MemoryLimitMultiplier <= 0 {
			t.Errorf("%s: multipliers = %v/%v, want both > 0",
				key, cfg.TimeLimitMultiplier, cfg.MemoryLimitMultiplier)
		}
	}
}

// Reset restores these same bytes, so the seeded config and the reset config
// must be the same document. A mismatch would mean "Reset to Default" produces
// something different from what a fresh install started with.
func TestShippedDefaultsMatchesResetBytes(t *testing.T) {
	got, err := ShippedDefaults()
	if err != nil {
		t.Fatalf("ShippedDefaults: %v", err)
	}

	for _, key := range lang.Keys() {
		raw, ok := lang.Default(key)
		if !ok {
			continue
		}
		var seeded LangConfig
		if err := json.Unmarshal(got[key], &seeded); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		var reset LangConfig
		if err := yaml.Unmarshal(raw, &reset); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if seeded.CompileCmd != reset.CompileCmd || seeded.Runtime != reset.Runtime {
			t.Errorf("%s: seeded compile/runtime = %q/%q, reset = %q/%q",
				key, seeded.CompileCmd, seeded.Runtime, reset.CompileCmd, reset.Runtime)
		}
	}
}
