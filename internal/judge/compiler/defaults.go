package compiler

import (
	"encoding/json"
	"fmt"

	"github.com/tahsinarafat/aioj/lang"
	"gopkg.in/yaml.v3"
)

// ShippedDefaults returns every config that ships in lang/*.yaml as the JSON
// document the language registry stores, keyed by language key.
//
// This is what a fresh install seeds from, so it must agree with what
// Reset-to-Default restores — both read the same embedded bytes, which is why
// this lives here rather than as a second table of defaults. It sits in
// `compiler` (not `lang`) because the conversion needs LangConfig, and having
// `lang` import `compiler` would close a cycle: compiler_test.go already reads
// lang/ to check that every binary a config names exists in the judge image.
//
// Multipliers default to 1.0 for the same reason the admin handlers do it: a
// zero multiplier renders as "0 s" on the problem page and tells a contestant
// nothing.
func ShippedDefaults() (map[string]json.RawMessage, error) {
	keys := lang.Keys()
	out := make(map[string]json.RawMessage, len(keys))
	for _, key := range keys {
		raw, ok := lang.Default(key)
		if !ok {
			continue
		}
		var cfg LangConfig
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("parse shipped lang/%s.yaml: %w", key, err)
		}
		if cfg.Key == "" {
			cfg.Key = key
		}
		if cfg.TimeLimitMultiplier <= 0 {
			cfg.TimeLimitMultiplier = 1.0
		}
		if cfg.MemoryLimitMultiplier <= 0 {
			cfg.MemoryLimitMultiplier = 1.0
		}
		doc, err := json.Marshal(&cfg)
		if err != nil {
			return nil, fmt.Errorf("encode shipped lang/%s.yaml: %w", key, err)
		}
		out[key] = doc
	}
	return out, nil
}
