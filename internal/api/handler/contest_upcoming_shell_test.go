package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

/**
 * An upcoming contest must be reachable without being open.
 *
 * The reported bug: clicking a contest that had not started yet answered
 * "Contest not found". The page already had a countdown for the `upcoming`
 * status, but nothing ever reached it — the read was refused, the page had no
 * .catch, and every rejection rendered as that one misleading string.
 *
 * The fix has to stop at the shell. A round that has not begun is public the way
 * an announcement is: its title, its times, its countdown. It is emphatically not
 * public the way a running round is: the statements, standings and booklet all
 * keep the strict gate, and the problem list is withheld rather than served with
 * a Solve button beside each letter.
 *
 * contest_access_test.go already pins the policy verdict for each of these cases.
 * What it cannot pin is the step after it: which gate a surface asks, and what
 * status a refusal becomes. That translation is where the original bug lived, so
 * these tests stop at the HTTP boundary instead of the accessLevel value.
 */

// upcomingAccess builds the handler access layer the gates hang off, wired to the
// same policy the real handlers use.
func upcomingAccess() *contestHandlerAccess {
	h := &contestHandlerAccess{}
	h.WithAccess(newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{}))
	return h
}

func requestFor(id string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/contests/"+id, nil)
}

// The shell gate is what makes the countdown reachable. If this fails, the page
// is back to "Contest not found" on a contest that exists.
func TestShellGateAcceptsAnUpcomingRound(t *testing.T) {
	h := upcomingAccess()
	rec := httptest.NewRecorder()

	viewer, ok := h.requireContestShellAccess(rec, requestFor("c1"), unstartedContest(), false)
	if !ok {
		t.Fatalf("shell gate refused an upcoming round: %d %s", rec.Code, rec.Body.String())
	}
	if !viewer.decision.IsUpcoming() {
		t.Fatal("the decision must say upcoming, so content is withheld")
	}
}

/**
 * The content gate must keep refusing it. This is the assertion that stops the
 * fix from becoming an exploit: every surface that hands out the round itself
 * asks requireContestAccess, so if upcoming ever satisfies it, the statements,
 * the booklet, the standings and submit all open hours before the start.
 */
func TestContentGateStillRefusesAnUpcomingRound(t *testing.T) {
	h := upcomingAccess()
	rec := httptest.NewRecorder()

	if _, ok := h.requireContestAccess(rec, requestFor("c1"), unstartedContest(), false); ok {
		t.Fatal("content gate must refuse an upcoming round")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 from the content gate", rec.Code)
	}
}

/**
 * Two surfaces ask for the round's content with the second argument set, so a
 * refusal must answer 404 rather than 403: answering 403 for a hidden round and
 * 200 for a visible one would turn the statements and the booklet into an
 * existence oracle for unpublished contests.
 *
 * This is the half the fix could silently break. Both call sites
 * (contest_problem.go and the PDF download) pass true, so if the shell gate ever
 * widened into the content gate these are the endpoints that would start
 * confirming a draft exists.
 */
func TestContentGateAnswersNotFoundForHiddenRounds(t *testing.T) {
	h := upcomingAccess()

	t.Run("a draft is not confirmed to exist", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if _, ok := h.requireContestAccess(rec, requestFor("c1"), draftContest(), true); ok {
			t.Fatal("content gate must not open a draft")
		}
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 so a draft stays unconfirmed", rec.Code)
		}
	})

	t.Run("an unstarted round is not confirmed to exist", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if _, ok := h.requireContestAccess(rec, requestFor("c1"), unstartedContest(), true); ok {
			t.Fatal("content gate must not open an upcoming round")
		}
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 so an unstarted round stays unconfirmed", rec.Code)
		}
	})

	// The shell, by contrast, is deliberately public: it must not 404 on the same
	// round, or the contest page goes back to rendering "Contest not found".
	t.Run("the shell of an unstarted round is not a 404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if _, ok := h.requireContestShellAccess(rec, requestFor("c1"), unstartedContest(), true); !ok {
			t.Fatalf("shell gate refused an upcoming round: %d %s", rec.Code, rec.Body.String())
		}
	})
}

// A draft is not an upcoming round: it is nobody's business until it is
// published, and both gates must refuse it. If this ever passes, an unpublished
// round is readable by strangers.
func TestBothGatesRefuseADraft(t *testing.T) {
	draft := draftContest()
	h := upcomingAccess()

	rec := httptest.NewRecorder()
	if _, ok := h.requireContestShellAccess(rec, requestFor("c1"), draft, false); ok {
		t.Fatal("the shell gate must not open a draft")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("shell status = %d, want 403 for a draft", rec.Code)
	}

	rec = httptest.NewRecorder()
	if _, ok := h.requireContestAccess(rec, requestFor("c1"), draft, false); ok {
		t.Fatal("the content gate must not open a draft")
	}
}

// A started round is unaffected: both gates open, which is what every surface
// already relied on.
func TestStartedRoundPassesBothGates(t *testing.T) {
	started := publishedContest()
	h := upcomingAccess()

	rec := httptest.NewRecorder()
	if _, ok := h.requireContestAccess(rec, requestFor("c1"), started, false); !ok {
		t.Fatalf("content gate refused a started round: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	if _, ok := h.requireContestShellAccess(rec, requestFor("c1"), started, false); !ok {
		t.Fatalf("shell gate refused a started round: %d %s", rec.Code, rec.Body.String())
	}
}
