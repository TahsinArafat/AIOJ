package checker

import (
	"sort"
	"strings"
)

// SortedChecker sorts both outputs line-by-line and compares them.
type SortedChecker struct{}

func (c *SortedChecker) Name() string { return "sorted" }

func (c *SortedChecker) Check(input, expected, actual []byte) *Result {
	expLines := sortLines(string(expected))
	actLines := sortLines(string(actual))
	if len(expLines) != len(actLines) {
		return &Result{Passed: false, Score: 0, Message: "line count mismatch"}
	}
	for i := range expLines {
		if expLines[i] != actLines[i] {
			return &Result{Passed: false, Score: 0, Message: "line mismatch after sorting"}
		}
	}
	return &Result{Passed: true, Score: 100, Message: "OK"}
}

func sortLines(s string) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	sort.Strings(lines)
	return lines
}
