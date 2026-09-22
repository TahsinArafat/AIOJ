package handler

import (
	"testing"
)

// The language tag guards a value that becomes part of a primary key and is
// echoed back to the frontend, so it must reject anything that isn't a plausible
// BCP-47 tag rather than silently storing arbitrary text.
func TestValidLanguage(t *testing.T) {
	valid := []string{"en", "bn", "vi", "pt-BR", "zh-Hans", "en-US"}
	for _, l := range valid {
		if !validLanguage(l) {
			t.Errorf("validLanguage(%q) = false, want true", l)
		}
	}

	invalid := []string{
		"",             // empty
		"e",            // too short
		"english",      // a word, not a tag
		"en US",        // space
		"en/../etc",    // path traversal attempt
		"../../secret", // traversal
		"<script>",     // injection shape
		"en_US",        // underscore separator is not BCP-47
	}
	for _, l := range invalid {
		if validLanguage(l) {
			t.Errorf("validLanguage(%q) = true, want false", l)
		}
	}
}
