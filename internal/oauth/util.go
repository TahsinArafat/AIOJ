package oauth

import (
	"encoding/json"
	"io"
	"strings"
)

func jsonDecode(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }

// sanitizeUsername lowercases and strips characters that would break URLs
// or collide with reserved paths. Falls back to a stable prefix.
func sanitizeUsername(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "user"
	}
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}
