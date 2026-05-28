package checker

import (
	"sort"
	"strings"
)

// UnorderedChecker compares outputs as multisets — order-independent, duplicate-aware.
type UnorderedChecker struct{}

func (c *UnorderedChecker) Name() string { return "unordered" }

func (c *UnorderedChecker) Check(input, expected, actual []byte) *Result {
	expTokens := sortedTokens(string(expected))
	actTokens := sortedTokens(string(actual))
	if len(expTokens) != len(actTokens) {
		return &Result{Passed: false, Score: 0, Message: "token count mismatch"}
	}
	for i := range expTokens {
		if expTokens[i] != actTokens[i] {
			return &Result{Passed: false, Score: 0, Message: "token mismatch (multiset comparison)"}
		}
	}
	return &Result{Passed: true, Score: 100, Message: "OK"}
}

func sortedTokens(s string) []string {
	tokens := strings.Fields(s)
	sort.Strings(tokens)
	return tokens
}
