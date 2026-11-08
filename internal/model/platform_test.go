package model

import "testing"

// "Remote" must be an explicit list, not "anything that is not local": an AI
// problem was routed to a remote platform called "ai" and came back as a
// System Error.
func TestIsRemotePlatform(t *testing.T) {
	cases := map[string]bool{
		"codeforces": true,
		"atcoder":    true,
		"cses":       true,
		"toph":       true,
		"qoj":        true,
		"local":      false,
		"ai":         false,
		"":           false,
		"sim":        false,
	}
	for source, want := range cases {
		if got := IsRemotePlatform(source); got != want {
			t.Errorf("IsRemotePlatform(%q) = %v, want %v", source, got, want)
		}
	}
}
