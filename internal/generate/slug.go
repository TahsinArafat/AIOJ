package generate

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// nonAlphanumRe matches any character that is not a letter, digit, or hyphen.
var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a title to a URL-safe slug (e.g. "Two Sum!" → "two-sum").
//
// Lives in its own file because ApplyService uses it when a proposal creates a
// brand-new problem. It previously sat next to the legacy synchronous
// generator, which has been removed.
func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 100 {
		s = s[:100]
	}
	if s == "" {
		s = uuid.New().String()[:8]
	}
	return s
}
