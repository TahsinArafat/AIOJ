package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * contestAccessPolicy is the single answer to "may this caller see this contest".
 *
 * It replaces canReadContest, a free function four handlers copied and five more
 * never called. The failure this table exists to prevent is concrete: a setter
 * marked a round invite-only and password-protected, and people who were never
 * invited opened it and submitted solutions. The password was stored, then
 * deleted before use, and compared against nothing; the group restriction ran on
 * two of seven surfaces.
 *
 * So the cases below are not decorative — each one is a door that was open.
 */

const testPasswordHash = "$2a$12$notarealhashbutnonempty"

// accessStub is the narrow store the policy asks. Both questions are settable so a
// test can place a caller inside or outside a group without a database.
type accessStub struct {
	staff bool
	// levels, when set, is answered per-level so a test can model a tester (staff,
	// but not a judge). staff=true with no levels means "any level".
	levels    []string
	groupOK   bool
	groupErr  error
	granted   bool
	grantErr  error
	groupSeen bool
}

func (s *accessStub) HasAccess(_ context.Context, _, _ string, required ...string) bool {
	if s.levels != nil {
		for _, held := range s.levels {
			for _, want := range required {
				if held == want {
					return true
				}
			}
		}
		return false
	}
	return s.staff
}

func (s *accessStub) CheckGroupRestriction(context.Context, string, string) (bool, error) {
	s.groupSeen = true
	return s.groupOK, s.groupErr
}

type grantStub struct {
	granted bool
	err     error
}

func (g grantStub) IsGranted(context.Context, string, string) (bool, error) {
	return g.granted, g.err
}

func publishedContest() *model.Contest {
	return &model.Contest{
		ID: "c1", Visible: true,
		StartTime: time.Now().Add(-time.Hour), EndTime: time.Now().Add(time.Hour),
		CreatedBy: "creator1",
	}
}

func draftContest() *model.Contest {
	c := publishedContest()
	c.Visible = false
	return c
}

func unstartedContest() *model.Contest {
	c := publishedContest()
	c.StartTime = time.Now().Add(time.Hour)
	return c
}

func passwordContest() *model.Contest {
	c := publishedContest()
	c.PasswordHash = testPasswordHash
	return c
}

func TestContestAccessPolicyDecide(t *testing.T) {
	user := &auth.Claims{UserID: "u", Role: "user"}

	cases := []struct {
		name    string
		contest *model.Contest
		claims  *auth.Claims
		stub    *accessStub
		grant   grantStub
		want    accessLevel
		wantErr bool
	}{
		// --- the public case still works -------------------------------------
		{name: "published and started is public", contest: publishedContest(), stub: &accessStub{groupOK: true}, want: accessGranted},

		// --- drafts and unstarted rounds -------------------------------------
		{name: "anonymous cannot read a draft", contest: draftContest(), stub: &accessStub{groupOK: true}, want: accessNone},
		{name: "plain user cannot read a draft", contest: draftContest(), claims: user, stub: &accessStub{groupOK: true}, want: accessNone},
		// An upcoming round is not "no access": it is public the way an
		// announcement is, which is what lets its page render a countdown. It is
		// still not granted — every content surface refuses it.
		{name: "anonymous sees an unstarted round as upcoming", contest: unstartedContest(), stub: &accessStub{groupOK: true}, want: accessNotStarted},
		{name: "plain user sees an unstarted round as upcoming", contest: unstartedContest(), claims: user, stub: &accessStub{groupOK: true}, want: accessNotStarted},
		{name: "nil contest", contest: nil, want: accessNone},

		// --- staff reach their own round -------------------------------------
		{name: "admin can read a draft", contest: draftContest(), claims: &auth.Claims{UserID: "a", Role: "admin"}, stub: &accessStub{}, want: accessGranted},
		{name: "creator can read a draft", contest: draftContest(), claims: &auth.Claims{UserID: "creator1", Role: "setter"}, stub: &accessStub{}, want: accessGranted},
		{name: "manager can read a draft", contest: draftContest(), claims: &auth.Claims{UserID: "m", Role: "setter"}, stub: &accessStub{staff: true}, want: accessGranted},
		{name: "judge can read an unstarted round", contest: unstartedContest(), claims: &auth.Claims{UserID: "j", Role: "setter"}, stub: &accessStub{staff: true}, want: accessGranted},
		{name: "unrelated setter cannot read a draft", contest: draftContest(), claims: &auth.Claims{UserID: "s2", Role: "setter"}, stub: &accessStub{}, want: accessNone},

		// --- the group hole: was unenforced on 5 of 7 surfaces ----------------
		{name: "group member may read", contest: publishedContest(), claims: user, stub: &accessStub{groupOK: true}, want: accessGranted},
		{name: "non-member of a restricted group is refused", contest: publishedContest(), claims: user, stub: &accessStub{groupOK: false}, want: accessNone},
		{name: "anonymous is refused a restricted group", contest: publishedContest(), stub: &accessStub{groupOK: false}, want: accessNone},

		// --- the password hole: the reported exploit --------------------------
		{
			name: "password round without a grant asks for the password", contest: passwordContest(),
			claims: user, stub: &accessStub{groupOK: true}, grant: grantStub{granted: false},
			want: accessPasswordRequired,
		},
		{
			name: "password round with a grant is open", contest: passwordContest(),
			claims: user, stub: &accessStub{groupOK: true}, grant: grantStub{granted: true},
			want: accessGranted,
		},
		{
			// An anonymous caller cannot hold a grant, so they must be stopped at the
			// prompt rather than let through.
			name: "anonymous never holds a grant", contest: passwordContest(),
			stub: &accessStub{groupOK: true}, grant: grantStub{granted: true},
			want: accessPasswordRequired,
		},
		{
			// Staff never unlock their own round.
			name: "staff bypass the password", contest: passwordContest(),
			claims: &auth.Claims{UserID: "m", Role: "setter"}, stub: &accessStub{staff: true},
			want: accessGranted,
		},

		// --- failure handling -------------------------------------------------
		{name: "group store error fails closed", contest: publishedContest(), claims: user, stub: &accessStub{groupErr: errors.New("db down")}, wantErr: true},
		{name: "grant store error fails closed", contest: passwordContest(), claims: user, stub: &accessStub{groupOK: true}, grant: grantStub{err: errors.New("db down")}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := tc.stub
			if stub == nil {
				stub = &accessStub{groupOK: true}
			}
			p := newContestAccessPolicy(stub, tc.grant)

			got, err := p.Decide(context.Background(), tc.contest, tc.claims, model.SurfaceParticipate)
			if tc.wantErr {
				if err == nil {
					t.Fatal("a store failure must surface, not become an access grant")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.level != tc.want {
				t.Fatalf("level = %v, want %v", got.level, tc.want)
			}
		})
	}
}

// A draft must never reveal that it is password-protected: the prompt is only
// reachable once the caller is otherwise entitled to the round. Otherwise "does
// this id exist and is it locked" is answered for anyone who guesses.
func TestDraftNeverRevealsPasswordState(t *testing.T) {
	c := draftContest()
	c.PasswordHash = testPasswordHash
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{})

	got, err := p.Decide(context.Background(), c, &auth.Claims{UserID: "nobody", Role: "user"}, model.SurfaceParticipate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NeedsPassword() {
		t.Fatal("an unreadable draft must not advertise that it has a password")
	}
	if got.Allowed() {
		t.Fatal("a draft must not be readable")
	}
}

// IsStaff decides redaction and the "can bypass the disabled booklet" override, so
// it has to be true for exactly the people who run the round.
func TestDecisionIsStaff(t *testing.T) {
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{})

	creator, _ := p.Decide(context.Background(), publishedContest(), &auth.Claims{UserID: "creator1", Role: "setter"}, model.SurfaceParticipate)
	if !creator.IsStaff() {
		t.Fatal("the contest creator is staff")
	}
	plain, _ := p.Decide(context.Background(), publishedContest(), &auth.Claims{UserID: "u", Role: "user"}, model.SurfaceParticipate)
	if plain.IsStaff() {
		t.Fatal("an ordinary contestant is not staff")
	}
}

/**
 * An unstarted round is readable as a shell and closed as content.
 *
 * The page reported "Contest not found" for a contest that had merely not begun,
 * because the only answer the policy had for an upcoming round was the same
 * accessNone a nonexistent one produced. accessNotStarted splits those: the page
 * renders the title, the times and a countdown, while every surface that hands out
 * the round itself still refuses.
 *
 * The two assertions below are the pair that matters. If upcoming ever becomes
 * Allowed(), statements, the booklet, standings and submit all open early — every
 * one of them asks this same policy.
 */
func TestUnstartedRoundIsShellOnly(t *testing.T) {
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{})

	got, err := p.Decide(context.Background(), unstartedContest(), &auth.Claims{UserID: "u", Role: "user"}, model.SurfaceParticipate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsUpcoming() {
		t.Fatal("an unstarted round must be reported as upcoming")
	}
	if got.Allowed() {
		t.Fatal("an unstarted round must not grant access to its content")
	}
}

// A round that has not begun must not answer "does this have a password" or "is
// this invite-only" either: there is nothing to unlock yet, so the prompt would be
// pure information for someone probing ids.
func TestUnstartedRoundRevealsNoOtherRestriction(t *testing.T) {
	locked := unstartedContest()
	locked.PasswordHash = testPasswordHash

	p := newContestAccessPolicy(&accessStub{groupOK: false}, grantStub{})
	got, err := p.Decide(context.Background(), locked, &auth.Claims{UserID: "u", Role: "user"}, model.SurfaceParticipate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NeedsPassword() {
		t.Fatal("an unstarted round must not advertise that it is password-protected")
	}
	if got.Allowed() {
		t.Fatal("an unstarted round must not be readable")
	}
	if !got.IsUpcoming() {
		t.Fatal("the wait state must survive both other restrictions")
	}
}

// An upcoming round is published but not open. The split between shell and content
// is what lets the page show a countdown instead of "Contest not found" without
// handing out the problem set.
func TestUpcomingRoundYieldsShellOnly(t *testing.T) {
	p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{})

	t.Run("anonymous gets the shell, not access", func(t *testing.T) {
		got, err := p.Decide(context.Background(), unstartedContest(), nil, model.SurfaceParticipate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.IsUpcoming() {
			t.Fatal("an unstarted round must be reported as upcoming")
		}
		if got.Allowed() {
			t.Fatal("upcoming must not count as access to the round's content")
		}
	})

	t.Run("an upcoming password round does not reveal that it is locked", func(t *testing.T) {
		c := unstartedContest()
		c.PasswordHash = testPasswordHash

		got, err := p.Decide(context.Background(), c, &auth.Claims{UserID: "u", Role: "user"}, model.SurfaceParticipate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.NeedsPassword() {
			t.Fatal("a round that has not started must not advertise a password")
		}
		if got.Allowed() {
			t.Fatal("an unstarted round must not be readable")
		}
	})

	t.Run("staff see their unstarted round as open", func(t *testing.T) {
		got, err := p.Decide(context.Background(), unstartedContest(), &auth.Claims{UserID: "creator1", Role: "setter"}, model.SurfaceParticipate)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Allowed() {
			t.Fatal("the creator must be able to open their own unstarted round")
		}
	})
}

// A tester may open a round but is not a judge: the difference decides whether
// they can see standings past the freeze.
func TestTesterIsStaffButNotJudge(t *testing.T) {
	// A tester holds exactly the tester permission: enough to open the round, not
	// enough to be treated as a judge.
	p := newContestAccessPolicy(&accessStub{levels: []string{model.ContestPermissionTester}, groupOK: true}, grantStub{})

	got, err := p.Decide(context.Background(), draftContest(), &auth.Claims{UserID: "t", Role: "setter"}, model.SurfaceParticipate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Allowed() || !got.IsStaff() {
		t.Fatal("a tester must be able to open the round")
	}
	if got.IsJudge() {
		t.Fatal("a tester must not hold judge privilege: that would expose frozen standings")
	}
}
