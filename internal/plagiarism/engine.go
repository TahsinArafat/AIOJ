package plagiarism

import (
	"regexp"
	"strings"
)

var (
	commentLineRegex  = regexp.MustCompile(`(?m)//.*$`)
	commentBlockRegex = regexp.MustCompile(`(?s)/\*.*?\*/`)
	stringRegex       = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)
	numRegex          = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	whiteRegex        = regexp.MustCompile(`\s+`)
)

// NormalizeCode strips comments, normalizes strings/numbers to tokens, and collapses whitespace.
func NormalizeCode(code string) string {
	code = commentLineRegex.ReplaceAllString(code, " ")
	code = commentBlockRegex.ReplaceAllString(code, " ")
	code = stringRegex.ReplaceAllString(code, " STR ")
	code = numRegex.ReplaceAllString(code, " NUM ")
	code = whiteRegex.ReplaceAllString(code, " ")
	return strings.TrimSpace(code)
}

// Tokenize splits normalized code into whitespace-delimited tokens.
func Tokenize(code string) []string {
	normalized := NormalizeCode(code)
	if normalized == "" {
		return []string{}
	}
	return strings.Fields(normalized)
}

// CompareCodes computes the structural similarity ratio (0.0–1.0) between two source code strings.
func CompareCodes(code1, code2 string) float64 {
	tokens1 := Tokenize(code1)
	tokens2 := Tokenize(code2)
	return LCSSimilarity(tokens1, tokens2)
}
