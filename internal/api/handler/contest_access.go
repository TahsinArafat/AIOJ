package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * This file is the single answer to "may this caller see this contest".
 *
 * It replaces a free function, canReadContest, that four handlers hand-copied and
 * five more never called at all. The result was a contest marked invite-only and
 * password-protected that anyone could open, read the statements of, and submit
 * to: the password was stored, then deleted before use (contest.go:317) and never
 * compared anywhere, and the group restriction ran on two of seven surfaces.
 *
 * The POLICY is an object rather than a function for one concrete reason: adding a
 * restriction is the operation that has already failed twice. With a free function
 * each new rule means touching every handler, and the ones that get missed are the
 * ones nobody is looking at (gym, virtual, editorial, the booklet, submit). Here a
 * new rule is one entry in Decide's ordered pipeline, and every surface gets it
 * because every surface asks the same object.
 */

// accessLevel is the outcome of the policy, not an HTTP status. Translating it to
// a status code is the handler's job, so a read of the statements can answer 404
// while an unlock answers 403 without the policy knowing about either.
type accessLevel int

const (
	// accessNone: the caller may not see the round. Covers a draft for
	// non-staff, group non-membership, and any store failure (fail closed).
	accessNone accessLevel = iota
	// accessNotStarted: the round is published but its start time is still in
	// the future. It is public in the way an announcement is — the title, the
	// times and a countdown — and closed in the way the round's content is.
	// Distinct from accessNone because the two used to be the same answer, and
	// that made an upcoming contest indistinguishable from a nonexistent one:
	// the page rendered "Contest not found" on a round that was simply waiting
	// to begin. Only the contest page itself may act on this; every content
	// surface (statements, booklet, standings, submit) still refuses it.
	accessNotStarted
	// accessPasswordRequired: the caller clears every other gate but has not
	// unlocked this round. Distinct from accessNone so the UI can prompt instead
	// of showing "not found", and so an anonymous visitor's existence probe still
	// cannot distinguish a password round from a public one until they hold a
	// grant.
	accessPasswordRequired
	accessGranted
)

// contestAccessDecision is a value object. Handlers read it; they never re-derive
// policy from the Contest struct, which is how the copies drifted apart.
type contestAccessDecision struct {
	level accessLevel
	// isStaff is "may run this round at all": admin, creator, manager, judge or
	// tester. It gates access and the booklet's disabled-override.
	isStaff bool
	// isJudge is the narrower privilege the scoreboard and the statement's spoiler
	// rules use: admin, manager or judge, NOT tester.
	//
	// The two are separate on purpose. Collapsing them would quietly widen judge
	// rights to testers — most visibly by letting a tester see standings past the
	// freeze, which is exactly the leak the freeze exists to prevent.
	isJudge bool
	// contentAllowed records what the CONTENT pipeline answered, which for a
	// spectate decision is not the same as whether the caller may watch. A public
	// board is watchable by somebody who cannot open a single statement, and the
	// scoreboard has to tell those two callers apart: a non-participant spectator
	// gets the board without the problem titles.
	contentAllowed bool
	// err carries a fail-closed store error out of the content steps. Not an
	// access level because it is not an answer — it is the absence of one.
	err error
}

func (d contestAccessDecision) Allowed() bool       { return d.level == accessGranted }
func (d contestAccessDecision) NeedsPassword() bool { return d.level == accessPasswordRequired }
func (d contestAccessDecision) IsStaff() bool       { return d.isStaff }

// IsJudge is the narrower privilege: admin, creator, manager or judge — not tester.
// Used where the answer unlocks something a tester must not have, such as seeing
// standings past the freeze or overriding a disabled booklet.
func (d contestAccessDecision) IsJudge() bool { return d.isJudge }

// IsUpcoming reports that the round is published but has not started. The caller
// may render the round's shell; it may not read its content.
func (d contestAccessDecision) IsUpcoming() bool { return d.level == accessNotStarted }

// contestAccessStore is the slice of the contest store the policy needs. Declared
// here, on the consumer side, so a test can satisfy it with two methods instead of
// the whole ContestStore — and so this file does not depend on postgres.
type contestAccessStore interface {
	HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool
	CheckGroupRestriction(ctx context.Context, contestID, userID string) (bool, error)
}

// contestGrantChecker is the publisher half of "did this user type the password".
// Satisfied by *postgres.ContestGrantStore.
type contestGrantChecker interface {
	IsGranted(ctx context.Context, contestID, userID string) (bool, error)
}

// contestAccessPolicy is the seam every contest surface goes through.
type contestAccessPolicy struct {
	contests contestAccessStore
	grants   contestGrantChecker
	// now is injectable so tests can place a contest before/after its window
	// without sleeping, and so the policy has one clock rather than each call
	// site calling time.Now separately.
	now func() time.Time
}

// newContestAccessPolicy builds the policy. A nil grant store is tolerated and
// treated as "nobody has unlocked anything", so handlers that predate grants (and
// tests that do not care) keep working without a password round becoming public.
func newContestAccessPolicy(cs contestAccessStore, gs contestGrantChecker) contestAccessPolicy {
	return contestAccessPolicy{contests: cs, grants: gs, now: time.Now}
}

// Decide is the whole policy, as an ordered pipeline:
//
//  1. nil contest            -> none
//  2. staff                  -> granted (they run the round; they never need to
//     unlock their own contest, and they must be able to
//     open it while it is still a draft)
//  3. draft                  -> none (an unpublished round is nobody's business)
//  4. published, not started -> not started (the shell is public, the content is not)
//  5. wrong group            -> none
//  6. password without grant -> password required
//  7. otherwise              -> granted
//
// Staff is answered before the visibility rule so a setter is not locked out of
// their own draft; visibility is answered before the password rule so a draft
// never reveals that it is password-protected. accessNotStarted is answered
// before the group and password rules for the same reason: a round that has not
// begun must not leak whether it is invite-only or locked, and there is nothing
// behind the answer to unlock yet.
func (p contestAccessPolicy) Decide(ctx context.Context, c *model.Contest, claims *auth.Claims) (contestAccessDecision, error) {
	return p.decide(ctx, c, claims, false)
}

// DecideSpectate answers the second question: "may this caller watch this round's
// scoreboard". It is the same ordered pipeline with steps 5 and 6 — group and
// password — deliberately skipped when the round publishes its board.
//
// Why a second axis exists at all: Decide answers "may this caller read this
// round", and hiding the board behind it made a whole category of round
// impossible. An onsite final is private to the people in the room and its board
// is the point — projected, streamed, pasted into a group chat. With one verdict
// the only way to publish the board was to publish the statements too.
//
// What is deliberately NOT relaxed, each for a concrete reason:
//
//   - Staff, draft and !Visible are answered before the split, so a draft's board
//     is still nobody's business and a rejected round has no board.
//   - accessNotStarted survives, so a public board cannot become the side channel
//     that reveals an invite-only round hours before it begins. After the start
//     the setter has chosen to publish the result and nothing is left to infer.
//   - When the board is not public the pipeline is identical to Decide, so the
//     participants-only case cannot drift from the content rules.
//
// isStaff/isJudge come out of the same computation, because the board needs the
// judge privilege for the freeze whichever axis admitted the reader.
func (p contestAccessPolicy) DecideSpectate(ctx context.Context, c *model.Contest, claims *auth.Claims) (contestAccessDecision, error) {
	return p.decide(ctx, c, claims, true)
}

// decide is the one pipeline. spectate only decides what happens after the
// questions both axes share have been answered: a public board turns what would
// have been a group refusal or a password prompt into a grant.
//
// Putting the flag here rather than in a wrapper is not a style choice. A wrapper
// that inspected Decide's verdict could not tell a group refusal from a store
// failure — both are accessNone, and one of them must stay closed — so the split
// has to happen where the reason is still known.
func (p contestAccessPolicy) decide(ctx context.Context, c *model.Contest, claims *auth.Claims, spectate bool) (contestAccessDecision, error) {
	if c == nil {
		return contestAccessDecision{}, nil
	}

	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	// 2. Staff. HasAccess must not be called with an empty user id: it would ask
	// the database about the empty string on every anonymous page load.
	isAdmin := claims != nil && claims.Role == "admin"
	isCreator := claims != nil && c.CreatedBy == userID
	hasAccess := func(levels ...string) bool {
		return userID != "" && p.contests.HasAccess(ctx, c.ID, userID, levels...)
	}
	isStaff := isAdmin || isCreator ||
		hasAccess(model.ContestPermissionManager, model.ContestPermissionJudge, model.ContestPermissionTester)
	// Judge privilege mirrors what the handlers asked before this refactor: a tester
	// is staff but not a judge, and must not see past a freeze.
	isJudge := isAdmin || isCreator ||
		hasAccess(model.ContestPermissionManager, model.ContestPermissionJudge)
	if isStaff {
		// contentAllowed is set explicitly, not defaulted: staff read the round, so
		// the board they watch is the full one. Leaving it false would strip the
		// problem titles off a setter's own scoreboard.
		return contestAccessDecision{level: accessGranted, isStaff: true, isJudge: isJudge, contentAllowed: true}, nil
	}

	// 3. Published but not started. Answering accessNone here is what made an
	// upcoming round look nonexistent; accessNotStarted says "wait", which only
	// the contest page itself turns into a countdown. Note this reads Visible,
	// which SetReviewState already mirrors from status, so a pending round is not
	// visible and stops here. Keeping one source (visible) rather than
	// re-deriving from status is deliberate: two sources for this question is
	// how the copies drifted.
	if !c.Visible {
		return contestAccessDecision{}, nil
	}
	if p.now().Before(c.StartTime) {
		return contestAccessDecision{level: accessNotStarted}, nil
	}

	// The split. Everything above is a question both axes ask and answer the same
	// way; everything below belongs to content alone.
	content := p.contentLevel(ctx, c, userID)
	if content.err != nil {
		return contestAccessDecision{}, content.err
	}

	decision := contestAccessDecision{
		level:   content.level,
		isJudge: isJudge,
		// Recorded here because only the board needs it, and only the board can get
		// it for free: a public board is watchable by a caller who cannot read the
		// round, and the scoreboard has to know which of those two it is serving so
		// it can strip the problem titles a spectator must not receive.
		contentAllowed: content.level == accessGranted,
	}

	// A round that publishes its board is watchable before the group and password
	// rules are consulted. This is the entire feature, and it is why the reason
	// cannot be recovered from the verdict afterwards: a group refusal and a store
	// failure are both "not granted", and one of them must stay closed.
	if spectate && c.IsScoreboardPublic() {
		decision.level = accessGranted
	}

	return decision, nil
}

// contentLevel is steps 4 and 5 of the pipeline — the group restriction and the
// password. Separated out so the spectate path can read the answer without
// duplicating the rules, since two copies of an access rule is the exact failure
// this file was written to end.
func (p contestAccessPolicy) contentLevel(ctx context.Context, c *model.Contest, userID string) contestAccessDecision {
	// 4. Group restriction. This used to run on GetByID and Register only, so a
	// non-member who guessed a contest id could read its statements, its PDF and
	// its standings, and submit to it.
	allowed, err := p.contests.CheckGroupRestriction(ctx, c.ID, userID)
	if err != nil {
		// Fail closed. A database error must not become an access grant.
		return contestAccessDecision{err: err}
	}
	if !allowed {
		return contestAccessDecision{}
	}

	// 5. Password. An anonymous caller cannot hold a grant, so this correctly
	// stops them at "password required" rather than letting them in.
	if c.PasswordHash != "" {
		granted := false
		if userID != "" && p.grants != nil {
			granted, err = p.grants.IsGranted(ctx, c.ID, userID)
			if err != nil {
				return contestAccessDecision{err: err}
			}
		}
		if !granted {
			return contestAccessDecision{level: accessPasswordRequired}
		}
	}

	return contestAccessDecision{level: accessGranted}
}

// ---------------------------------------------------------------------------
// Shared handler plumbing
// ---------------------------------------------------------------------------

// contestViewer resolves the caller once per request: the decision, the claims,
// and the two derived facts every contest handler needs (staff, and whether the
// answer is "show me a password prompt").
type contestViewer struct {
	decision contestAccessDecision
	claims   *auth.Claims
}

// requireContestSpectateAccess is the board's gate. It is requireContestAccess
// with the spectate question asked instead of the content one, so a round with a
// public scoreboard is watchable by a caller who cannot read a word of it.
//
// An unstarted round is refused here, for the same reason the content gate
// refuses it: a board that is readable hours before the start is a side channel
// telling the world an invite-only round exists, which is exactly what
// accessNotStarted exists to prevent. Note that this is the *handler's* decision
// and has to match the policy's — the policy answering accessNotStarted is not by
// itself enough, because an earlier version of this function treated "upcoming" as
// "let them through" and would have served the board before the start.
func (h *contestHandlerAccess) requireContestSpectateAccess(
	w http.ResponseWriter,
	r *http.Request,
	c *model.Contest,
	notFoundWhenDenied bool,
) (contestViewer, bool) {
	claims := middleware.GetUserClaims(r)

	decision, err := h.access.DecideSpectate(r.Context(), c, claims)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return contestViewer{}, false
	}

	switch {
	case decision.Allowed():
		return contestViewer{decision: decision, claims: claims}, true
	case notFoundWhenDenied:
		http.Error(w, "contest not found", http.StatusNotFound)
		return contestViewer{}, false
	default:
		http.Error(w, "forbidden", http.StatusForbidden)
		return contestViewer{}, false
	}
}

// requireContestAccess is the one place a contest handler turns a policy decision
// into an HTTP response. It returns false when the handler must stop.
//
// notFoundWhenDenied exists because the surfaces disagree, for good reasons: the
// problem statement and the booklet answer 404 so a draft's existence is not
// confirmed to a stranger (contest_problem.go and DownloadPDF already did this
// and the tests pin it), while the scoreboard answers 403.
//
// An unstarted round is refused here, deliberately. Every caller of this function
// reads *content* — statements, the booklet, standings, submissions — and none of
// it exists before the start. The contest page itself wants the shell instead and
// calls requireContestShellAccess.
func (h *contestHandlerAccess) requireContestAccess(
	w http.ResponseWriter,
	r *http.Request,
	c *model.Contest,
	notFoundWhenDenied bool,
) (contestViewer, bool) {
	return h.resolveContestAccess(w, r, c, notFoundWhenDenied, false)
}

// requireContestShellAccess is requireContestAccess for a surface that renders the
// round's metadata rather than its content.
//
// Only the contest page uses it: on an upcoming round it needs the title, the
// times and the countdown, and refusing there is what produced "Contest not
// found" on a contest that existed and had merely not begun. Accepting an
// unstarted round here does not open the round — the problems, statements,
// standings and submit all go through requireContestAccess, which still refuses.
func (h *contestHandlerAccess) requireContestShellAccess(
	w http.ResponseWriter,
	r *http.Request,
	c *model.Contest,
	notFoundWhenDenied bool,
) (contestViewer, bool) {
	return h.resolveContestAccess(w, r, c, notFoundWhenDenied, true)
}

func (h *contestHandlerAccess) resolveContestAccess(
	w http.ResponseWriter,
	r *http.Request,
	c *model.Contest,
	notFoundWhenDenied bool,
	allowUpcoming bool,
) (contestViewer, bool) {
	claims := middleware.GetUserClaims(r)

	decision, err := h.access.Decide(r.Context(), c, claims)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return contestViewer{}, false
	}

	switch {
	case decision.Allowed():
		return contestViewer{decision: decision, claims: claims}, true
	case decision.IsUpcoming() && allowUpcoming:
		return contestViewer{decision: decision, claims: claims}, true
	case decision.NeedsPassword():
		// The body is a machine-readable code, not prose, so the contest page can
		// render an unlock prompt without parsing an error string.
		//
		// The scoreboard flag is added for one concrete reason: a round with a
		// public board is deliberately unlisted, so a spectator who was handed its
		// link arrives here and would otherwise be told only that the round is
		// private. It is safe to say — they already hold the id, and the board is
		// available to anyone — and without it the published board has no route a
		// stranger can follow. Not sent for a round whose board is participant-only,
		// where it would be a hint about nothing.
		body := map[string]string{
			"error": "password_required",
			"code":  "password_required",
		}
		if c.IsScoreboardPublic() {
			body["scoreboard_public"] = "true"
		}
		respondJSON(w, http.StatusForbidden, body)
		return contestViewer{}, false
	case notFoundWhenDenied:
		http.Error(w, "contest not found", http.StatusNotFound)
		return contestViewer{}, false
	default:
		http.Error(w, "forbidden", http.StatusForbidden)
		return contestViewer{}, false
	}
}

// contestHandlerAccess carries the policy for handlers that are not
// ContestHandler. Embedded so ContestHandler, ContestProblemHandler and the rest
// pick up requireContestAccess without each declaring the method.
type contestHandlerAccess struct {
	access contestAccessPolicy
}

// WithAccess attaches the contest access policy. Handlers built without one (unit
// tests exercising something else) get a policy backed by whatever store they
// already hold; see NewContestHandler.
func (h *contestHandlerAccess) WithAccess(p contestAccessPolicy) { h.access = p }

// rebuildWithGrants swaps in a policy that can also consult password grants.
//
// Every handler that reads or writes contest data builds its policy with a nil
// grant checker by default (safe: a password round fails closed). Wiring the real
// store is what makes such a round openable at all, so main calls this on each of
// them. Kept on the embed because the rebuild is identical everywhere - only the
// store differs, and the caller passes its own.
func (h *contestHandlerAccess) rebuildWithGrants(cs contestAccessStore, gs contestGrantChecker) {
	h.access = newContestAccessPolicy(cs, gs)
}

// errContestAccessDenied is the sentinel submission paths return after writing a
// refusal. Exists so a handler can distinguish "I refused, response is written"
// from a real fault without a bool out-parameter.
var errContestAccessDenied = errors.New("contest access denied")
