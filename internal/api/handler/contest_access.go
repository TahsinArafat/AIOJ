package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
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
	// spectate decision is not the same as whether the caller may watch: a public
	// board is watchable by somebody who cannot open a single statement.
	//
	// The scoreboard used to read this to decide whether to strip the problem
	// titles off the board. It no longer does — the board carries letters only, for
	// every caller — so nothing in production consumes this field today. It is kept
	// because it is the content axis's answer on a spectate request, which the
	// public-board case is precisely what makes non-obvious, and it is what the
	// spectate tests assert the split on; a caller that needs "may this spectator
	// also read the round" has one place to ask.
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
// here, on the consumer side, so a test can satisfy it with a few methods instead
// of the whole ContestStore — and so this file does not depend on postgres.
type contestAccessStore interface {
	HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool
	CheckGroupRestriction(ctx context.Context, contestID, userID string) (bool, error)
	// IsParticipant answers "did this user enter this round" — registered, or has
	// already submitted. The only thing that makes AccessParticipants mean
	// anything: before it was consulted here, selecting that audience stored a
	// value no gate ever read, so a setter who believed their problems were
	// entrants-only had published them to the world.
	IsParticipant(ctx context.Context, contestID, userID string) (bool, error)
}

// contestReadAccess is the seam for a handler that embeds the access policy and
// also reads the round itself: the two roles, composed rather than restated.
//
// Four handlers (gym, group, editorial, virtual) each held the whole 31-method
// ContestStore for exactly this pair of needs, which is what made them
// impossible to test without a contest store.
type contestReadAccess interface {
	store.ContestReader
	store.ContestAccessStore
}

// contestGrantChecker is the publisher half of "did this user type the password".
// Satisfied by *postgres.ContestGrantStore.
type contestGrantChecker interface {
	IsGranted(ctx context.Context, contestID, userID string) (bool, error)
}

// contestGrantRevoker is the grant store *plus* the one write a contest update
// needs: changing the password drops the entitlements the old one bought.
//
// Kept separate from contestGrantChecker because only the password-change path
// calls this. Every other WithGrants caller wants the read half, and handing it
// RevokeAll would let it invalidate a round it has no business touching.
type contestGrantRevoker interface {
	contestGrantChecker
	RevokeAll(ctx context.Context, contestID string) error
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

// Decide answers "may this caller reach this surface of this round", as one
// ordered pipeline:
//
//  1. nil contest            -> none
//  2. staff                  -> granted (they run the round; they never need to
//     unlock their own contest, and they must be able to
//     open it while it is still a draft)
//  3. draft                  -> none (an unpublished round is nobody's business)
//  4. published, not started -> not started (the shell is public, the content is not)
//  5. group/password         -> none / password_required, UNLESS this surface's
//     audience is public
//  6. otherwise              -> granted
//
// Staff is answered before the visibility rule so a setter is not locked out of
// their own draft; visibility is answered before the password rule so a draft
// never reveals that it is password-protected. accessNotStarted is answered
// before the group and password rules for the same reason: a round that has not
// begun must not leak whether it is invite-only or locked, and there is nothing
// behind the answer to unlock yet.
//
// The surface is a parameter rather than a second entry point because the reason
// the split lives inside this pipeline is unchanged from when there was exactly
// one such surface (the scoreboard): a wrapper that inspected the verdict could
// not tell a group refusal from a store failure — both are accessNone, and one of
// them must stay closed — so the override has to happen where the reason is still
// known. Generalising the entry point from a bool to a surface name is what makes
// the sixth surface cost one line instead of a sixth copy of this function.
func (p contestAccessPolicy) Decide(ctx context.Context, c *model.Contest, claims *auth.Claims, surface string) (contestAccessDecision, error) {
	return p.decide(ctx, c, claims, surface)
}

// decide is the pipeline itself. The surface only decides the last step — every
// question above it is asked and answered the same way for every surface, which
// is what keeps a new surface from having to restate the draft, visibility,
// group and password rules (and from getting one of them subtly wrong).
func (p contestAccessPolicy) decide(ctx context.Context, c *model.Contest, claims *auth.Claims, surface string) (contestAccessDecision, error) {
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

	// The split. Everything above is a question every surface asks and answers the
	// same way; everything below belongs to the surface's own audience.
	content := p.contentLevel(ctx, c, userID)
	if content.err != nil {
		return contestAccessDecision{}, content.err
	}

	decision := contestAccessDecision{
		level:          content.level,
		isJudge:        isJudge,
		contentAllowed: content.level == accessGranted,
	}

	// This surface's audience, resolved once. An unset surface resolves to
	// AccessOpen — the §4 content rules and nothing more — which is what makes an
	// empty access_config mean exactly today's behaviour.
	switch c.AccessConfig.Audience(surface) {
	case model.AccessPublic:
		// The feature, generalised from the scoreboard. A public surface is
		// reachable before the group and password rules are consulted, so a
		// spectator who never held the password can read exactly this one surface
		// and nothing else.
		//
		// Note what this does NOT do: it does not touch any other surface's
		// audience, and it cannot. Each gate call site passes its own surface, so
		// "the board is public" can never widen the statements — the bug this
		// whole axis exists to prevent.
		decision.level = accessGranted
	case model.AccessParticipants:
		// Only people who entered the round. This is the one audience that
		// RESTRICTS, and it is deliberately applied on top of the content verdict
		// rather than instead of it: `content` already answered the draft, group
		// and password questions, and the only case that reaches here with the
		// surface still open is a caller who cleared all of them.
		//
		// Which surfaces may ask this is decided in the model, not here — see
		// SurfaceMayRequireParticipation. SurfaceSubmit is excluded because
		// requiring an entrant to submit deadlocks the first submission of every
		// round, and Audience() has already coerced that pair to AccessOpen, so
		// this branch is unreachable for submit by construction.
		if !decision.Allowed() {
			// Already refused by the group or password rule. Nothing to narrow.
			break
		}
		// An anonymous caller cannot have entered, so the answer is known without
		// asking — and asking is not merely wasteful here. user_id is a uuid
		// column, so IsParticipant(ctx, id, "") is not a query that finds nothing:
		// it is a query Postgres rejects with `invalid input syntax for type
		// uuid: ""`, which this function turns into a 500. Guarding on the empty
		// id is what keeps a signed-out visitor's request from becoming a server
		// error on a round they are merely not entitled to.
		if userID == "" {
			decision.level = accessNone
			break
		}
		participant, err := p.contests.IsParticipant(ctx, c.ID, userID)
		if err != nil {
			// Fail closed, like every other store fault in this pipeline: a
			// database error must not become an access grant.
			return contestAccessDecision{}, err
		}
		if !participant {
			decision.level = accessNone
		}
	case model.AccessStaff:
		// Organizers only. Reached only for a caller who is not already staff (the
		// staff branch returned above), so the answer is a flat refusal, and it is
		// accessNone rather than a password prompt: there is no credential that
		// would admit this caller, and offering one would be a lie.
		decision.level = accessNone
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

// requireContestSurfaceAccess is the one place a contest handler turns a policy
// decision into an HTTP response. It returns false when the handler must stop.
//
// Every gated route names its surface. That is deliberate: the surface decides
// which audience applies, so a route that forgot to name one would silently fall
// back to the round's base gate — which is safe, but invisibly so, and the whole
// point of this refactor is that a surface's audience is a visible decision at
// the call site rather than something inferred.
//
// notFoundWhenDenied exists because the surfaces disagree, for good reasons: the
// problem statement and the booklet answer 404 so a draft's existence is not
// confirmed to a stranger (contest_problem.go and DownloadPDF already did this
// and the tests pin it), while the scoreboard answers 403.
//
// An unstarted round is refused here, deliberately. Every caller of this function
// reads a surface whose content does not exist before the start. The contest page
// itself wants the shell instead and calls requireContestShellAccess.
func (h *contestHandlerAccess) requireContestSurfaceAccess(
	w http.ResponseWriter,
	r *http.Request,
	c *model.Contest,
	surface string,
	notFoundWhenDenied bool,
) (contestViewer, bool) {
	return h.resolveContestAccess(w, r, c, surface, notFoundWhenDenied, false)
}

// requireContestShellAccess is requireContestSurfaceAccess for a surface that
// renders the round's metadata rather than its content.
//
// Only the contest page uses it: on an upcoming round it needs the title, the
// times and the countdown, and refusing there is what produced "Contest not
// found" on a contest that existed and had merely not begun. Accepting an
// unstarted round here does not open the round — the problems, statements,
// standings and submit all go through requireContestSurfaceAccess, which still
// refuses.
//
// contestPageAccess is what the contest page needs to render, which is more than
// one verdict: it carries the round's shell AND its problem list, and those are
// two surfaces with two audiences.
type contestPageAccess struct {
	access contestViewer
	// showProblems is the problem-list surface's own answer, already resolved
	// against staff privilege. The handler renders the list or an empty slice from
	// this, and never re-derives it from viewer.decision.
	showProblems bool
}

// requireContestPageAccess is the contest page's gate. The page is admitted when
// EITHER surface it serves is reachable, and each surface's content is decided
// separately.
//
// Why this is not just requireContestShellAccess: gating the page on the base gate
// alone made a published problem list unreachable on the one request that renders
// it. On a password round, `problem_list: public` was honoured by the per-problem
// stats endpoint but the page answered 403 before the list was ever consulted —
// so a setter who made their list public watched nothing change on the page a
// spectator would actually open. A setting that only works on a side door is
// worse than no setting, because it looks like it works.
//
// The refusal shape is unchanged: a password round that publishes nothing still
// answers password_required so the page can prompt, and a draft still answers
// forbidden. Publishing one surface does not turn the round into a public one.
func (h *contestHandlerAccess) requireContestPageAccess(
	w http.ResponseWriter,
	r *http.Request,
	c *model.Contest,
	notFoundWhenDenied bool,
) (contestPageAccess, bool) {
	claims := middleware.GetUserClaims(r)

	// The base verdict decides the shell and staff/judge privilege.
	base, err := h.access.Decide(r.Context(), c, claims, model.SurfaceParticipate)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return contestPageAccess{}, false
	}

	// The list verdict decides one list, and is independent of the base.
	list, err := h.access.Decide(r.Context(), c, claims, model.SurfaceProblemList)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return contestPageAccess{}, false
	}

	// Staff and an unstarted round are answered by the base pipeline before any
	// audience is consulted, so this branch is the shell behaviour that existed
	// before this gate: a setter opens their own draft, and an unstarted round
	// opens for its countdown.
	//
	// An unstarted round shows no list whatever its audience says. That is the
	// policy's accessNotStarted rule, and it is re-applied here because this gate
	// admits the page on the list's answer too: without the guard, publishing a
	// problem list would serve it hours before the start, which is the side channel
	// accessNotStarted exists to prevent.
	upcoming := base.IsUpcoming()
	if base.Allowed() || upcoming {
		return contestPageAccess{
			access:       contestViewer{decision: base, claims: claims},
			showProblems: list.Allowed() && !upcoming,
		}, true
	}

	// The list is published, so the page opens for it even though the round's
	// content stays closed. Note the decision returned is the BASE's, so
	// contentAllowed is false and nothing downstream mistakes this caller for a
	// participant — the only surface they were granted is the one named here.
	if list.Allowed() {
		return contestPageAccess{
			access:       contestViewer{decision: base, claims: claims},
			showProblems: true,
		}, true
	}

	// Neither surface is reachable: report the base's refusal, which describes the
	// round rather than the list, and keeps every existing refusal shape.
	switch {
	case base.NeedsPassword():
		// The body is a machine-readable code, not prose, so the contest page can
		// render an unlock prompt without parsing an error string.
		//
		// The published-surface list is added for one concrete reason: a round that
		// publishes a surface is deliberately unlisted, so a spectator who was
		// handed its link arrives here and would otherwise be told only that the
		// round is private. It is safe to say — they already hold the id, and those
		// surfaces are available to anyone — and without it a published surface has
		// no route a stranger can follow.
		contestPasswordRequired(w, c)
	case notFoundWhenDenied:
		http.Error(w, "contest not found", http.StatusNotFound)
	default:
		http.Error(w, "forbidden", http.StatusForbidden)
	}
	return contestPageAccess{}, false
}

func (h *contestHandlerAccess) resolveContestAccess(
	w http.ResponseWriter,
	r *http.Request,
	c *model.Contest,
	surface string,
	notFoundWhenDenied bool,
	allowUpcoming bool,
) (contestViewer, bool) {
	claims := middleware.GetUserClaims(r)

	decision, err := h.access.Decide(r.Context(), c, claims, surface)
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
		// The published-surface flag is added for one concrete reason: a round that
		// publishes a surface is deliberately unlisted, so a spectator who was
		// handed its link arrives here and would otherwise be told only that the
		// round is private. It is safe to say — they already hold the id, and that
		// surface is available to anyone — and without it the published surface has
		// no route a stranger can follow. Not sent for a round that publishes
		// nothing, where it would be a hint about nothing.
		//
		// Renamed from scoreboard_public to published_surfaces when the board
		// stopped being the only surface that could be public: the reader wants to
		// know what it may follow a link to, and naming one surface would have gone
		// stale (and been wrong) the first time a round published its booklet
		// instead. scoreboard_public is still sent when the board is one of them, so
		// the existing standings page keeps working unchanged.
		contestPasswordRequired(w, c)
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
// pick up requireContestSurfaceAccess without each declaring the method.
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
