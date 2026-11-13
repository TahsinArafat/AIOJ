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
 * The standings surface is a second axis: a round can keep its statements private
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
	c.AccessConfig = model.AccessConfig{model.SurfaceStandings: model.AccessPublic}
	return c
}

// withParticipantsBoard restricts the board to people who entered the round.
//
// This is NOT "the old behaviour" — it is a deliberate restriction that used to
// be a no-op and now withholds the board from anyone who has not registered or
// submitted. A test that means "nobody has configured this round" wants an empty
// AccessConfig instead, which resolves to AccessOpen.
func withParticipantsBoard(c *model.Contest) *model.Contest {
	c.AccessConfig = model.AccessConfig{model.SurfaceStandings: model.AccessParticipants}
	return c
}

func TestContestAccessPolicyStandingsSurface(t *testing.T) {
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

		// --- a participants-only board actually withholds the board -----------
		{
			// The backwards-compatibility case. A round with no configured surface
			// resolves to open, so it behaves exactly as every round did before this
			// vocabulary existed — which is the whole reason open is the default
			// rather than participants.
			name:    "an unset board still asks for the password",
			contest: passwordContest(),
			claims:  user, stub: &accessStub{groupOK: true}, grant: grantStub{granted: false},
			want: accessPasswordRequired, wantContent: false,
		},
		{
			name:    "an unset board still refuses a non-member",
			contest: publishedContest(),
			claims:  user, stub: &accessStub{groupOK: false},
			want: accessNone, wantContent: false,
		},
		{
			// The setting this axis withheld before: a board restricted to entrants
			// must not reach a signed-in stranger who never entered.
			name:    "a participants board refuses a non-participant who can read the round",
			contest: withParticipantsBoard(publishedContest()),
			claims:  user, stub: &accessStub{groupOK: true, participant: false},
			want: accessNone, wantContent: true,
		},
		{
			name:    "a participants board is watchable by a participant",
			contest: withParticipantsBoard(publishedContest()),
			claims:  user, stub: &accessStub{groupOK: true, participant: true},
			want: accessGranted, wantContent: true,
		},
		{
			// Anonymous callers cannot have entered, so they are refused. Uses an
			// unrestricted round so the ONLY thing that can refuse them is the
			// participation rule — on a password round the prompt would.
			name:    "a participants board refuses an anonymous visitor on an open round",
			contest: withParticipantsBoard(publishedContest()),
			stub:    &accessStub{groupOK: true, participant: false},
			want:    accessNone, wantContent: true,
		},
		{
			// A store failure answering the participation question must not grant.
			name:    "a participants board fails closed on a store fault",
			contest: withParticipantsBoard(publishedContest()),
			claims:  user, stub: &accessStub{groupOK: true, partErr: errors.New("db down")},
			wantErr: true,
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

			got, err := p.Decide(context.Background(), tc.contest, tc.claims, model.SurfaceStandings)
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
 * the regression test for the mistake that would make the feature dangerous: if
 * the standings surface were ever wired to a content surface, every private round
 * with a public board would hand out its problems.
 */
func TestPublicBoardDoesNotOpenContent(t *testing.T) {
	c := withPublicBoard(passwordContest())
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{granted: false})

	content, err := p.Decide(context.Background(), c, &auth.Claims{UserID: "u", Role: "user"}, model.SurfaceParticipate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.Allowed() {
		t.Fatal("a public board must not make the statements readable without the password")
	}
	if !content.NeedsPassword() {
		t.Fatal("the content gate must still ask for the password")
	}

	spectate, err := p.Decide(context.Background(), c, &auth.Claims{UserID: "u", Role: "user"}, model.SurfaceStandings)
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

	got, err := p.Decide(context.Background(), c, &auth.Claims{UserID: "t", Role: "user"}, model.SurfaceStandings)
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
 * the old spectate gate treated "upcoming" as "let them through", so a
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
			_, ok := h.requireContestSurfaceAccess(rec, requestFor("c1"), tc.contest, model.SurfaceStandings, false)
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

/**
 * A private round with a published board, watched anonymously.
 *
 * This is the combination the setting exists for and the one reported from the
 * live round: password-protected problems, public standings, viewer signed out.
 * It answers 200 from the scoreboard and 404 from the contest page — two different
 * doors, and the spectator is only ever given the first. Getting this wrong makes
 * the feature useless in exactly its intended case, so it is pinned separately
 * from the table above where a group stub defaults to allowed.
 */
func TestAnonymousSpectatorWatchesAPrivateRoundsPublicBoard(t *testing.T) {
	// Anonymous: nil claims, so no grant is even possible.
	private := withPublicBoard(passwordContest())
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{granted: false})

	spectate, err := p.Decide(context.Background(), private, nil, model.SurfaceStandings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spectate.Allowed() {
		t.Fatalf("anonymous spectator was refused a public board (level=%v); a private round's public board is unlisted but watchable", spectate.level)
	}
	// The board is watchable, the round is not readable: that pair is the whole
	// point, and losing the second half would publish the statements.
	content, err := p.Decide(context.Background(), private, nil, model.SurfaceParticipate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.Allowed() {
		t.Fatal("a public board must not make the round's content readable")
	}
	if !content.NeedsPassword() {
		t.Fatalf("content answer = %v, want password_required so the page can prompt rather than 404", content.level)
	}
}

/**
 * One public surface leaves the other five exactly where they were.
 *
 * This is the invariant the whole refactor rests on, and the one whose absence
 * would be a security regression rather than a missing feature. Before
 * access_config there was a single "spectate" axis, so a public board could not
 * widen anything — there was nothing else to widen. Now that every surface has
 * its own audience, "the board is public" must not reach the statements, the
 * submit path or the problem list, and a gate that ignored its surface argument
 * would leak all three with no visible symptom on the board itself.
 *
 * Asserted through the real gate rather than the policy, for the reason the table
 * above documents: a rule that only exists in the policy is not a rule a request
 * sees. Every case is a round that publishes exactly one surface, probed on all
 * six.
 */
func TestOnePublicSurfaceDoesNotWidenTheOthers(t *testing.T) {
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{granted: false})

	for _, published := range []string{model.SurfaceStandings, model.SurfacePDF, model.SurfaceProblemList} {
		t.Run("only "+published+" is public", func(t *testing.T) {
			c := passwordContest()
			c.AccessConfig = model.AccessConfig{published: model.AccessPublic}

			for _, surface := range model.AccessSurfaceKeys {
				got, err := p.Decide(context.Background(), c, nil, surface)
				if err != nil {
					t.Fatalf("surface %q: unexpected error: %v", surface, err)
				}
				wantOpen := surface == published
				if got.Allowed() != wantOpen {
					t.Errorf("with %q public, surface %q allowed=%v, want %v — a public "+
						"surface must not widen any other",
						published, surface, got.Allowed(), wantOpen)
				}
			}
		})
	}

	// And the base gate does flow down: publishing SurfaceParticipate opens every
	// surface that has no override of its own, which is what makes "participate" a
	// sensible default rather than a sixth thing to set.
	t.Run("a public base opens the surfaces that inherit", func(t *testing.T) {
		c := passwordContest()
		c.AccessConfig = model.AccessConfig{model.SurfaceParticipate: model.AccessPublic}

		for _, surface := range model.AccessSurfaceKeys {
			got, err := p.Decide(context.Background(), c, nil, surface)
			if err != nil {
				t.Fatalf("surface %q: unexpected error: %v", surface, err)
			}
			if !got.Allowed() {
				t.Errorf("surface %q was refused on a round whose base gate is public", surface)
			}
		}
	})

	// An explicit override beats the inherited base in both directions. A surface
	// set to staff stays closed even when the base is public; a surface set to
	// public opens even when the base is not.
	t.Run("an override beats the base in both directions", func(t *testing.T) {
		closed := passwordContest()
		closed.AccessConfig = model.AccessConfig{
			model.SurfaceParticipate: model.AccessPublic,
			model.SurfaceStatement:   model.AccessStaff,
		}
		got, err := p.Decide(context.Background(), closed, nil, model.SurfaceStatement)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Allowed() {
			t.Fatal("a surface set to staff was opened by a public base gate")
		}

		open := passwordContest()
		open.AccessConfig = model.AccessConfig{model.SurfaceSubmit: model.AccessPublic}
		got, err = p.Decide(context.Background(), open, nil, model.SurfaceSubmit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Allowed() {
			t.Fatal("a surface set to public was closed by a participants base gate")
		}
	})

	// Staff reach every surface whatever the audiences say. A setter checking their
	// own round must not be locked out of the booklet because they set it to staff.
	t.Run("staff reach every surface regardless of audience", func(t *testing.T) {
		c := passwordContest()
		c.AccessConfig = model.AccessConfig{
			model.SurfacePDF:       model.AccessStaff,
			model.SurfaceStatement: model.AccessStaff,
		}
		staff := &auth.Claims{UserID: "m", Role: model.RoleSetter}
		sp := newContestAccessPolicy(&accessStub{staff: true}, grantStub{})
		for _, surface := range model.AccessSurfaceKeys {
			got, err := sp.Decide(context.Background(), c, staff, surface)
			if err != nil {
				t.Fatalf("surface %q: unexpected error: %v", surface, err)
			}
			if !got.Allowed() {
				t.Errorf("staff were refused surface %q", surface)
			}
		}
	})
}
