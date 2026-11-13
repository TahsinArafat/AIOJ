package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Watching a round is not reading it.
 *
 * DecideSpectate is the second axis: a round can keep its statements private
 * (password, group) and still publish its scoreboard. The cases below are the
 * boundary between those two answers, and each one is a way the feature goes
 * wrong if the split is drawn in the wrong place.
 *
 * The concrete round this exists for: an onsite final, or a company hiring
 * round. The competitors are in a room, the password keeps everyone else out of
 * the problems, and the board is on a projector for the audience — or pasted
 * into a chat link for people who were never in the room. Before this, publishing
 * that board meant publishing the statements, because there was one verdict.
 */

// withPublicBoard marks a contest as publishing its scoreboard.
func withPublicBoard(c *model.Contest) *model.Contest {
	c.ScoreboardVisibility = model.ScoreboardPublic
	return c
}

// withParticipantsBoard is the default a round has before anyone touches the
// setting, spelled out so a test that means "the old behaviour" says so.
func withParticipantsBoard(c *model.Contest) *model.Contest {
	c.ScoreboardVisibility = model.ScoreboardParticipants
	return c
}

func TestContestAccessPolicyDecideSpectate(t *testing.T) {
	user := &auth.Claims{UserID: "u", Role: "user"}

	cases := []struct {
		name    string
		contest *model.Contest
		claims  *auth.Claims
		stub    *accessStub
		grant   grantStub
		want    accessLevel
		// wantContent is the CONTENT verdict for the same caller, which the decision
		// carries separately so the scoreboard can strip problem titles from a
		// spectator who cannot read the round.
		wantContent bool
		wantErr     bool
	}{
		// --- the feature: a locked round with a public board ------------------
		{
			// The whole point. An anonymous visitor who never held the password, and
			// who the content policy stops at the prompt, may watch.
			name:    "public board is watchable without the password",
			contest: withPublicBoard(passwordContest()),
			stub:    &accessStub{groupOK: true}, grant: grantStub{granted: false},
			want: accessGranted, wantContent: false,
		},
		{
			// Same, signed in but not a participant.
			name:    "a plain user watches a password round's public board",
			contest: withPublicBoard(passwordContest()),
			claims:  user, stub: &accessStub{groupOK: true}, grant: grantStub{granted: false},
			want: accessGranted, wantContent: false,
		},
		{
			// A group-restricted round is the harder case: its content refusal is
			// accessNone, indistinguishable from a draft's. If the split were drawn
			// after the verdict rather than before it, this case would stay closed.
			name:    "public board of a group-restricted round is watchable",
			contest: withPublicBoard(publishedContest()),
			claims:  user, stub: &accessStub{groupOK: false},
			want: accessGranted, wantContent: false,
		},
		{
			// Someone who CAN read the round keeps the full board, titles included.
			name:    "a participant watching a public board may read the round too",
			contest: withPublicBoard(publishedContest()),
			claims:  user, stub: &accessStub{groupOK: true},
			want: accessGranted, wantContent: true,
		},
		{
			name:    "a granted contestant watching a public board may read the round",
			contest: withPublicBoard(passwordContest()),
			claims:  user, stub: &accessStub{groupOK: true}, grant: grantStub{granted: true},
			want: accessGranted, wantContent: true,
		},
		{
			// Staff run the round, so their board is the full one whatever the
			// setting says. Leaving contentAllowed false here would strip the titles
			// off a setter's own scoreboard.
			name:    "staff watching a public board may read the round",
			contest: withPublicBoard(passwordContest()),
			claims:  &auth.Claims{UserID: "m", Role: "setter"}, stub: &accessStub{staff: true},
			want: accessGranted, wantContent: true,
		},

		// --- the default: nothing changes for a participants-only board -------
		{
			// The backwards-compatibility case. Every round that existed before this
			// column had this behaviour, and it has to be identical to Decide.
			name:    "participants-only board still asks for the password",
			contest: withParticipantsBoard(passwordContest()),
			claims:  user, stub: &accessStub{groupOK: true}, grant: grantStub{granted: false},
			want: accessPasswordRequired, wantContent: false,
		},
		{
			name:    "participants-only board still refuses a non-member",
			contest: withParticipantsBoard(publishedContest()),
			claims:  user, stub: &accessStub{groupOK: false},
			want: accessNone, wantContent: false,
		},
		{
			// The zero value behaves like participants: a contest built by a caller
			// that predates the column, or a fixture that omits it, must not publish
			// its board by accident.
			name:    "an unset scoreboard_visibility is participant-only",
			contest: passwordContest(),
			claims:  user, stub: &accessStub{groupOK: true}, grant: grantStub{granted: false},
			want: accessPasswordRequired, wantContent: false,
		},
		{
			name:    "participants-only public board is watchable by a participant",
			contest: withParticipantsBoard(publishedContest()),
			claims:  user, stub: &accessStub{groupOK: true},
			want: accessGranted, wantContent: true,
		},
		{
			name:    "anonymous watches a participants-only public round",
			contest: withParticipantsBoard(publishedContest()),
			stub:    &accessStub{groupOK: true},
			want:    accessGranted, wantContent: true,
		},

		// --- what a public board must NOT open -------------------------------
		{
			// A draft's board is nobody's business. Answered before the split, so a
			// public-board flag cannot publish an unproposed round by the side door.
			name:    "a public board does not expose a draft",
			contest: withPublicBoard(draftContest()),
			claims:  user, stub: &accessStub{groupOK: true},
			want: accessNone,
		},
		{
			name:    "a public board does not expose a rejected round",
			contest: withPublicBoard(draftContest()),
			stub:    &accessStub{groupOK: true},
			want:    accessNone,
		},
		{
			// The inference guard. A public board must not become the side channel
			// that tells the world an invite-only round exists hours before it
			// begins — the same leak HasPassword is blanked in GetByID to avoid.
			name:    "a public board stays closed before the start",
			contest: withPublicBoard(unstartedContest()),
			claims:  user, stub: &accessStub{groupOK: true},
			want: accessNotStarted,
		},
		{
			name:    "a public board stays closed before the start even to anon",
			contest: withPublicBoard(unstartedContest()),
			stub:    &accessStub{groupOK: true},
			want:    accessNotStarted,
		},
		{name: "nil contest", contest: nil, want: accessNone},

		// --- failures stay closed --------------------------------------------
		{
			// A database error must not become a grant merely because the board is
			// public: "we could not answer" is not "yes".
			name:    "a store failure fails closed even for a public board",
			contest: passwordContest(), wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := tc.stub
			if stub == nil {
				stub = &accessStub{groupOK: true, groupErr: errors.New("db down")}
			}
			p := newContestAccessPolicy(stub, tc.grant)

			got, err := p.DecideSpectate(context.Background(), tc.contest, tc.claims)
			if tc.wantErr {
				if err == nil {
					t.Fatal("a store failure must surface, not become a watchable board")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.level != tc.want {
				t.Fatalf("level = %v, want %v", got.level, tc.want)
			}
			if got.contentAllowed != tc.wantContent {
				t.Fatalf("content_allowed = %v, want %v (this decides whether the "+
					"board carries problem titles)", got.contentAllowed, tc.wantContent)
			}
		})
	}
}

/**
 * The two axes must be reachable in one direction only.
 *
 * A public board opens the scoreboard; it must not open the statements. This is
 * the regression test for the mistake that would make the feature dangerous — if
 * DecideSpectate were ever wired to a content surface, every private round with a
 * public board would hand out its problems.
 */
func TestPublicBoardDoesNotOpenContent(t *testing.T) {
	c := withPublicBoard(passwordContest())
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{granted: false})

	content, err := p.Decide(context.Background(), c, &auth.Claims{UserID: "u", Role: "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.Allowed() {
		t.Fatal("a public board must not make the statements readable without the password")
	}
	if !content.NeedsPassword() {
		t.Fatal("the content gate must still ask for the password")
	}

	spectate, err := p.DecideSpectate(context.Background(), c, &auth.Claims{UserID: "u", Role: "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spectate.Allowed() {
		t.Fatal("the board must be watchable")
	}
}

// A public board must not widen judge privilege: a tester is staff but not a
// judge, and the freeze is enforced off IsJudge. This pins that the spectate path
// reports the same privilege as the content path rather than defaulting it.
func TestPublicBoardKeepsJudgePrivilegeNarrow(t *testing.T) {
	c := withPublicBoard(publishedContest())
	p := newContestAccessPolicy(&accessStub{levels: []string{model.ContestPermissionTester}}, grantStub{})

	got, err := p.DecideSpectate(context.Background(), c, &auth.Claims{UserID: "t", Role: "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Allowed() {
		t.Fatal("a tester may watch their round's board")
	}
	if got.IsJudge() {
		t.Fatal("a tester must not hold judge privilege: that would expose frozen standings")
	}
}

/**
 * The handler gate must refuse what the policy refuses.
 *
 * This test exists because the policy-only tests above did not catch a real bug:
 * requireContestSpectateAccess treated "upcoming" as "let them through", so a
 * published-but-unstarted round served its board to anyone. The policy was
 * correct the whole time — the handler simply did not enforce its answer.
 *
 * So the assertions below go through the handler, not the policy. A rule that
 * only exists in the policy is not a rule a request actually sees.
 */
func TestSpectateHandlerEnforcesPolicy(t *testing.T) {
	cases := []struct {
		name        string
		contest     *model.Contest
		wantOK      bool
		description string
	}{
		{
			// The bug. An unstarted round must not leak its board.
			name: "an upcoming public board is refused", contest: withPublicBoard(unstartedContest()),
			wantOK:      false,
			description: "a public board must not become a side channel before the start",
		},
		{
			// The feature, for contrast: once started, the board opens.
			name: "a started public board is served", contest: withPublicBoard(passwordContest()),
			wantOK: true,
		},
		{
			name: "a draft's public board is refused", contest: withPublicBoard(draftContest()),
			wantOK:      false,
			description: "publishing a board must not publish an unproposed round",
		},
		{
			// The backwards-compatible case: no board flag, password round, no grant.
			name: "a participants-only password round is refused", contest: passwordContest(),
			wantOK:      false,
			description: "the default must keep the board behind the password",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &contestHandlerAccess{}
			h.rebuildWithGrants(&accessStub{groupOK: true}, grantStub{granted: false})

			rec := httptest.NewRecorder()
			_, ok := h.requireContestSpectateAccess(rec, requestFor("c1"), tc.contest, false)
			if ok != tc.wantOK {
				t.Fatalf("gate admitted=%v, want %v (code=%d body=%q) — %s",
					ok, tc.wantOK, rec.Code, rec.Body.String(), tc.description)
			}
			if !tc.wantOK && rec.Code != http.StatusForbidden {
				t.Fatalf("refusal code = %d, want %d", rec.Code, http.StatusForbidden)
			}
		})
	}
}
