package postgres

import (
	"strings"
	"testing"
)

// A problem that is still in play in a contest is not part of the archive — the
// same rule Codeforces applies to a round's problems until the round ends. The
// condition lives in one constant so the problem list, its filtered variant and
// search cannot drift apart; this pins what it has to contain.
func TestArchivedProblemSQLExcludesUnfinishedContests(t *testing.T) {
	if !strings.Contains(archivedProblemSQL, "p.status = 'published'") {
		t.Error("the archive must require an admin-approved problem, not merely a visible one")
	}
	if !strings.Contains(archivedProblemSQL, "NOT EXISTS") {
		t.Error("the archive must exclude problems that are not over yet")
	}
	if !strings.Contains(archivedProblemSQL, "contest_problems") {
		t.Error("the exclusion must be driven by contest membership")
	}
	if !strings.Contains(archivedProblemSQL, "end_time > NOW()") {
		t.Error("only a contest that has not ended should hide its problems")
	}
}
