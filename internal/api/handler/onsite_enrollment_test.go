package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * An onsite team must be able to open the round it was generated for.
 *
 * This is the regression test for the failure setters actually hit: they
 * generated credentials, the teams signed in successfully, and then every page
 * answered "forbidden" — rendered as "Contest not found". Login worked, so
 * nothing looked broken until the competitors were already in the room.
 *
 * Two things were wrong, and both are pinned below:
 *
 *  1. Login auto-REGISTERED the team but never GRANTED it the round. The access
 *     policy answers the password question from contest_access_grants and never
 *     consults contest_registrations, so enrolling in a password round changed
 *     nothing. The setter created these accounts; making a competitor type a
 *     password nobody gave them is the bug.
 *
 *  2. The enrollment call sat inside the first-login branch, so a team that had
 *     already signed in once never took it. With the grant added, that would have
 *     locked out precisely the teams that had been waiting longest.
 */

// enrollingOnsiteStub records what login asked the store to do.
type enrollingOnsiteStub struct {
	user         *model.OnsiteBatchUser
	autoRegister [][2]string // (contestID, userID) pairs
}

func (s *enrollingOnsiteStub) CreateBatch(context.Context, string, []model.BatchUserRequest) ([]model.OnsiteBatchUser, error) {
	return nil, nil
}
func (s *enrollingOnsiteStub) GetByUsername(context.Context, string) (*model.OnsiteBatchUser, error) {
	return s.user, nil
}
func (s *enrollingOnsiteStub) MarkUsed(context.Context, string, string) error { return nil }
func (s *enrollingOnsiteStub) ListByContest(context.Context, string) ([]model.OnsiteBatchUser, error) {
	return nil, nil
}
func (s *enrollingOnsiteStub) DeleteByContest(context.Context, string) error    { return nil }
func (s *enrollingOnsiteStub) DeleteByID(context.Context, string, string) error { return nil }
func (s *enrollingOnsiteStub) AutoRegister(_ context.Context, contestID, userID string) error {
	s.autoRegister = append(s.autoRegister, [2]string{contestID, userID})
	return nil
}

/**
 * A team that has ALREADY signed in must still be enrolled on every later login.
 *
 * IsUsed/UsedBy are what make this the regression case: that team takes the
 * "linked account" branch, which is exactly where the enrollment call used to be
 * missing from. stubs_email's stubUserForEV is reused rather than hand-rolling
 * the twenty-method UserStore: it returns nil for an unknown username, which is
 * what routes login into the onsite branch.
 */
func TestOnsiteLogin_EnrollsOnEveryLogin(t *testing.T) {
	contestID := uuid.New().String()
	userID := uuid.New().String()
	const password = "team-password"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	usedBy := userID
	onsite := &enrollingOnsiteStub{
		user: &model.OnsiteBatchUser{
			ID:           uuid.New().String(),
			ContestID:    contestID,
			Username:     "team_abc12345",
			PasswordHash: hash,
			IsUsed:       true, // already signed in before — the regression case
			UsedBy:       &usedBy,
		},
	}
	// The linked account exists in users, and no user carries the onsite
	// username, so login falls through to the onsite store.
	users := &stubUserForEV{users: map[string]*model.User{
		userID: {ID: userID, Username: "team_abc12345", Role: "contestant"},
	}}

	h := NewAuthHandler(users, &countingRefreshStore{}, nil, onsite, nil,
		auth.NewJWTManager("test-secret-at-least-32-bytes-long!!", time.Minute, time.Hour),
		nil, nil, nil, nil, "", "")

	body, _ := json.Marshal(map[string]string{"username": "team_abc12345", "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if len(onsite.autoRegister) == 0 {
		t.Fatalf("a returning onsite team was never enrolled (status=%d body=%q) — "+
			"this is the bug that left participants on \"Contest not found\"",
			rec.Code, rec.Body.String())
	}
	got := onsite.autoRegister[0]
	if got[0] != contestID || got[1] != userID {
		t.Fatalf("enrolled (%q, %q), want (%q, %q)", got[0], got[1], contestID, userID)
	}
}

/**
 * The path a returning team ACTUALLY takes.
 *
 * This is the case the previous test missed, and the reason the earlier fix
 * appeared to do nothing on the live site.
 *
 * The first sign-in creates a users row carrying the SAME password hash as the
 * onsite credential. From the second sign-in onward, the ordinary
 * users.GetByUsername lookup therefore matches and verifies, and Login returns
 * from that branch — the onsite branch below it is never reached. So enrollment
 * that lives only in the onsite branch runs exactly once, ever, and every later
 * login takes the early return.
 *
 * Here the user row has a real password hash, so login legitimately takes the
 * ordinary branch. Enrollment must still happen.
 */
func TestOnsiteLogin_EnrollsWhenUserRowShadowsCredential(t *testing.T) {
	contestID := uuid.New().String()
	userID := uuid.New().String()
	const password = "team-password"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	usedBy := userID
	onsite := &enrollingOnsiteStub{
		user: &model.OnsiteBatchUser{
			ID: uuid.New().String(), ContestID: contestID,
			Username: "team_shadow1", PasswordHash: hash,
			IsUsed: true, UsedBy: &usedBy,
		},
	}
	// The shadowing row: same username, same password hash. This is what the
	// onsite branch itself created on first sign-in.
	users := &stubUserForEV{users: map[string]*model.User{
		userID: {
			ID: userID, Username: "team_shadow1", Role: "contestant",
			PasswordHash: hash,
		},
	}}

	h := NewAuthHandler(users, &countingRefreshStore{}, nil, onsite, nil,
		auth.NewJWTManager("test-secret-at-least-32-bytes-long!!", time.Minute, time.Hour),
		nil, nil, nil, nil, "", "")

	body, _ := json.Marshal(map[string]string{"username": "team_shadow1", "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	if len(onsite.autoRegister) == 0 {
		t.Fatal("login took the ordinary user branch and never enrolled the team — " +
			"this is why a team that had signed in once stayed locked out of its round " +
			"while login kept succeeding")
	}
	if got := onsite.autoRegister[0]; got[0] != contestID || got[1] != userID {
		t.Fatalf("enrolled (%q, %q), want (%q, %q)", got[0], got[1], contestID, userID)
	}
}

/**
 * A credential belonging to a different account must not grant anything.
 *
 * The username is UNIQUE so this should not arise, but the guard costs nothing
 * and stops a reused handle from inheriting another team's round.
 */
func TestOnsiteLogin_DoesNotEnrollIntoAnotherTeamsCredential(t *testing.T) {
	const password = "team-password"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	otherUser := uuid.New().String()
	onsite := &enrollingOnsiteStub{
		user: &model.OnsiteBatchUser{
			ID: uuid.New().String(), ContestID: uuid.New().String(),
			Username: "team_reused", PasswordHash: hash,
			IsUsed: true, UsedBy: &otherUser, // belongs to somebody else
		},
	}
	me := uuid.New().String()
	users := &stubUserForEV{users: map[string]*model.User{
		me: {ID: me, Username: "team_reused", Role: "contestant", PasswordHash: hash},
	}}

	h := NewAuthHandler(users, &countingRefreshStore{}, nil, onsite, nil,
		auth.NewJWTManager("test-secret-at-least-32-bytes-long!!", time.Minute, time.Hour),
		nil, nil, nil, nil, "", "")

	body, _ := json.Marshal(map[string]string{"username": "team_reused", "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if len(onsite.autoRegister) != 0 {
		t.Fatalf("enrolled %v into a credential owned by another account", onsite.autoRegister)
	}
}
