package postgres

import (
	"strings"
	"testing"
)

// A problem that is being used by a contest open right now is not part of the
// archive — the same rule Codeforces applies to a round's problems until the round
// ends. The condition lives in one constant so the problem list, its filtered
// variant and search cannot drift apart; this pins what it has to contain.
func TestArchivedProblemSQLExcludesUnfinishedContests(t *testing.T) {
	if !strings.Contains(archivedProblemSQL, "p.status = 'published'") {
		t.Error("the archive must require an admin-approved problem, not merely a visible one")
	}
	if !strings.Contains(archivedProblemSQL, "NOT EXISTS") {
		t.Error("the archive must exclude problems that are not over yet")
	}
	if strings.Contains(archivedProblemSQL, "NOT EXISTS") &&
		!strings.Contains(unreleasedContestSQL, "end_time > NOW()") {
		t.Error("only a contest that has not ended should hide its problems")
	}
}

/**
 * The predicate must withhold a problem only while somebody who is not supposed
 * to see it could actually reach it. Two earlier forms were too broad, and both
 * cost real catalogue entries:
 *
 *   - with no lower bound, a round scheduled for next week removed its problems
 *     from the archive the moment they were attached, days early;
 *   - with no `visible` guard, an unpublished draft round — which nobody can
 *     enter, and which conceals nothing — pinned a published problem out of the
 *     archive indefinitely.
 *
 * The conditions are asserted on the predicate rather than by standing up a
 * database so they cannot be quietly dropped; the end-to-end behaviour is pinned
 * by TestUnreleasedContestRequiresAnOpenRound below.
 */
func TestUnreleasedContestRequiresAnOpenRound(t *testing.T) {
	for _, want := range []string{"c.visible", "c.start_time <= NOW()", "c.end_time > NOW()"} {
		if !strings.Contains(unreleasedContestSQL, want) {
			t.Errorf("the predicate must be bounded by %q, got: %s", want, unreleasedContestSQL)
		}
	}
	if strings.Contains(unreleasedContestSQL, "c.start_time > NOW()") {
		t.Error("an upcoming round is not open yet and must not hide its problems")
	}
}
