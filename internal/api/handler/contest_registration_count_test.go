package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

/**
 * The contest page's participant count has a source.
 *
 * The client read `registration_count` and `is_registered` off the GetByID response
 * with `?? 0` / `?? false` fallbacks, and the handler sent neither — so the
 * "Participants" row rendered a hardcoded 0 no matter how many people had entered,
 * and the "You are registered" badge could not appear for anyone, including the
 * person who had just registered. The types file documented the omission as known.
 *
 * Asserted on the response body, because that is the contract the page reads: a
 * count that exists in the store but not in the payload is the bug.
 *
 * Two rules beyond "send the number":
 *
 *   - A round that does not require registration reports 0, not the size of an
 *     empty table. Register answers 400 on such a round, so there is no roster and
 *     a zero would read as "nobody entered" rather than "there is nothing to
 *     enter". The page only renders the row for registration-required rounds
 *     anyway.
 *   - No store wired means the fields are still present and zero. A handler built
 *     without the optional collaborator (every existing test) must not start
 *     failing, and the page's fallback already handles a zero.
 */
func TestContestGetByIDReportsRegistration(t *testing.T) {
	db := openHandlerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	problemStore := postgres.NewProblemStore(db)
	registrationStore := postgres.NewRegistrationStore(db)
	h := NewContestHandler(contestStore, postgres.NewRatingStore(db), problemStore, userStore, nil).
		WithRegistrations(registrationStore)

	owner := &model.User{
		ID: uuid.New().String(), Username: "reg_" + uuid.New().String()[:8],
		Email:        "reg_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	entrant := &model.User{
		ID: uuid.New().String(), Username: "ent_" + uuid.New().String()[:8],
		Email:        "ent_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleUser,
	}
	if err := userStore.Create(ctx, entrant); err != nil {
		t.Fatalf("create entrant: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", entrant.ID)
	}()

	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	mk := func(title string, registrationRequired bool) *model.Contest {
		return &model.Contest{
			ID: uuid.New().String(), Title: title, Type: "acm", Format: "acm",
			StartTime: start, EndTime: start.Add(3 * time.Hour),
			CreatedBy: owner.ID, Status: model.ContestStatusPublished, Visible: true,
			IsRated: true, RatedForMin: 0, RatedForMax: 9999,
			RegistrationRequired: registrationRequired,
		}
	}

	open := mk("Open "+uuid.New().String()[:8], true)
	unrestricted := mk("Unrestricted "+uuid.New().String()[:8], false)
	for _, c := range []*model.Contest{open, unrestricted} {
		c := c
		if err := contestStore.Create(ctx, c); err != nil {
			t.Fatalf("create %s: %v", c.Title, err)
		}
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
	}

	if err := registrationStore.Register(ctx, open.ID, entrant.ID); err != nil {
		t.Fatalf("register entrant: %v", err)
	}
	// Registered on the unrestricted round too, to prove the roster is not consulted
	// there: the count must be 0 because the round has no roster, not because the
	// table happens to be empty.
	if err := registrationStore.Register(ctx, unrestricted.ID, entrant.ID); err != nil {
		t.Fatalf("register on unrestricted: %v", err)
	}

	get := func(c *model.Contest, claims *auth.Claims) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/contests/"+c.ID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", c.ID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		if claims != nil {
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
		}
		rec := httptest.NewRecorder()
		h.GetByID(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GetByID = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body
	}

	t.Run("the count reflects the roster", func(t *testing.T) {
		body := get(open, nil)
		// JSON numbers decode to float64 through any.
		if got := body["registration_count"]; got != float64(1) {
			t.Fatalf("registration_count = %v, want 1 — the page renders this as the "+
				"participant count and it was a hardcoded 0", got)
		}
	})

	t.Run("a registrant is told they are registered", func(t *testing.T) {
		body := get(open, &auth.Claims{UserID: entrant.ID, Role: model.RoleUser})
		if got := body["is_registered"]; got != true {
			t.Fatalf("is_registered = %v, want true — the badge cannot appear for the "+
				"person who registered", got)
		}
	})

	t.Run("a different signed-in user is not", func(t *testing.T) {
		other := &model.User{
			ID: uuid.New().String(), Username: "oth_" + uuid.New().String()[:8],
			Email:        "oth_" + uuid.New().String()[:8] + "@example.com",
			PasswordHash: "hashed", Role: model.RoleUser,
		}
		if err := userStore.Create(ctx, other); err != nil {
			t.Fatalf("create other: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", other.ID)

		body := get(open, &auth.Claims{UserID: other.ID, Role: model.RoleUser})
		if got := body["is_registered"]; got != false {
			t.Fatalf("is_registered = %v for a non-registrant, want false", got)
		}
		if got := body["registration_count"]; got != float64(1) {
			t.Fatalf("registration_count = %v, want 1 — one person is registered", got)
		}
	})

	t.Run("an anonymous caller is never registered", func(t *testing.T) {
		body := get(open, nil)
		if got := body["is_registered"]; got != false {
			t.Fatalf("is_registered = %v for an anonymous caller, want false", got)
		}
	})

	t.Run("a round without registration reports no roster", func(t *testing.T) {
		body := get(unrestricted, &auth.Claims{UserID: entrant.ID, Role: model.RoleUser})
		if got := body["registration_count"]; got != float64(0) {
			t.Fatalf("registration_count = %v on a round that does not require "+
				"registration, want 0 — Register answers 400 there, so there is no roster", got)
		}
		if got := body["is_registered"]; got != false {
			t.Fatalf("is_registered = %v, want false", got)
		}
	})

	t.Run("the fields are present without the store wired", func(t *testing.T) {
		// Every existing test builds this handler with no registration store, and the
		// page's fallback is `?? 0`. The keys must still be there rather than absent,
		// or a client cannot tell "not wired" from an older server.
		bare := NewContestHandler(contestStore, postgres.NewRatingStore(db), problemStore, userStore, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/contests/"+open.ID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", open.ID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		bare.GetByID(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GetByID = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got, ok := body["registration_count"]; !ok || got != float64(0) {
			t.Fatalf("registration_count = %v (present=%v), want 0 present", got, ok)
		}
		if got, ok := body["is_registered"]; !ok || got != false {
			t.Fatalf("is_registered = %v (present=%v), want false present", got, ok)
		}
	})
}

/**
 * The About tab's organizer roster is staff-facing data on a public page.
 *
 * `ListPermissions` is mounted in the authenticated group with NO access check of
 * its own, so any signed-in user can already read a round's full staff list. That
 * is why this is not a new endpoint: the roster rides along with GetByID, where the
 * access policy has already decided whether the caller may read the round at all.
 *
 * Three rules, and each is a way it leaks or lies:
 *
 *   - Testers are excluded. They are staff for access purposes but are not part of
 *     the round's public face, and naming them publishes a role the round does not
 *     advertise.
 *   - The creator is included even when the permissions table has no row for them.
 *     Create inserts one, and a round created by an older build may not have it —
 *     without this a round's own author can be missing from its About page.
 *   - An unstarted round withholds the roster entirely. The shell is public before
 *     the start and the content is not, and "who is running this" is not shell: it
 *     is the same inference accessNotStarted exists to prevent.
 */
func TestContestGetByIDWithholdsJudgesFromNonReaders(t *testing.T) {
	db := openHandlerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	problemStore := postgres.NewProblemStore(db)
	h := NewContestHandler(contestStore, postgres.NewRatingStore(db), problemStore, userStore, nil)

	owner := &model.User{
		ID: uuid.New().String(), Username: "own_" + uuid.New().String()[:8],
		Email:        "own_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	judge := &model.User{
		ID: uuid.New().String(), Username: "jud_" + uuid.New().String()[:8],
		Email:        "jud_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleUser,
	}
	if err := userStore.Create(ctx, judge); err != nil {
		t.Fatalf("create judge: %v", err)
	}
	tester := &model.User{
		ID: uuid.New().String(), Username: "tst_" + uuid.New().String()[:8],
		Email:        "tst_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleUser,
	}
	if err := userStore.Create(ctx, tester); err != nil {
		t.Fatalf("create tester: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", judge.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", tester.ID)
	}()

	past := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	live := &model.Contest{
		ID: uuid.New().String(), Title: "Live " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: past, EndTime: past.Add(3 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished, Visible: true,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
		PasswordHash: testPasswordHash,
	}
	upcoming := &model.Contest{
		ID: uuid.New().String(), Title: "Soon " + uuid.New().String()[:8],
		Type: "acm", Format: "acm",
		StartTime: time.Now().UTC().Add(2 * time.Hour), EndTime: time.Now().UTC().Add(4 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished, Visible: true,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
	}
	for _, c := range []*model.Contest{live, upcoming} {
		c := c
		if err := contestStore.Create(ctx, c); err != nil {
			t.Fatalf("create %s: %v", c.Title, err)
		}
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
	}
	for _, c := range []*model.Contest{live, upcoming} {
		for _, p := range []struct {
			user  *model.User
			level string
		}{
			{judge, model.ContestPermissionJudge},
			{tester, model.ContestPermissionTester},
		} {
			if err := contestStore.AddPermission(ctx, c.ID, p.user.ID, p.level); err != nil {
				t.Fatalf("add permission: %v", err)
			}
		}
	}

	body := func(c *model.Contest, claims *auth.Claims) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/contests/"+c.ID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", c.ID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		if claims != nil {
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
		}
		rec := httptest.NewRecorder()
		h.GetByID(rec, req)
		// A password round refuses an anonymous caller outright, which is the correct
		// answer and the one that proves the roster is not on a side channel.
		if rec.Code != http.StatusOK && rec.Code != http.StatusForbidden {
			t.Fatalf("GetByID = %d, want 200 or 403 (body=%q)", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("decode: %v", err)
			}
		}
		return out
	}

	names := func(v any) []string {
		out := []string{}
		rows, _ := v.([]any)
		for _, r := range rows {
			m, _ := r.(map[string]any)
			if u, ok := m["username"].(string); ok {
				out = append(out, u)
			}
		}
		return out
	}

	t.Run("a granted caller sees the judge and not the tester", func(t *testing.T) {
		// Granted so the page is admitted: the point is which staff are named, not
		// whether the round opens.
		b := body(live, &auth.Claims{UserID: owner.ID, Role: model.RoleSetter})
		got := names(b["judges"])
		if !testListHas(got, judge.Username) {
			t.Errorf("judges = %v, want it to include %s", got, judge.Username)
		}
		if testListHas(got, tester.Username) {
			t.Errorf("judges = %v, must not include the tester %s", got, tester.Username)
		}
		if !testListHas(got, owner.Username) {
			t.Errorf("judges = %v, want it to include the creator %s", got, owner.Username)
		}
	})

	t.Run("an unstarted round withholds the roster from a visitor", func(t *testing.T) {
		// A signed-in NON-staff caller on an unstarted round. The shell is public
		// before the start and the content is not, and "who is running this" is not
		// shell — it is the same inference accessNotStarted exists to prevent.
		//
		// Staff are the contrast case, asserted below: they are granted at an earlier
		// step, so IsUpcoming() is false for them and they see their own round's
		// roster. Testing this with the creator would have asserted nothing, because
		// their decision is accessGranted.
		bystander := &model.User{
			ID: uuid.New().String(), Username: "byr_" + uuid.New().String()[:8],
			Email:        "byr_" + uuid.New().String()[:8] + "@example.com",
			PasswordHash: "hashed", Role: model.RoleUser,
		}
		if err := userStore.Create(ctx, bystander); err != nil {
			t.Fatalf("create bystander: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", bystander.ID)

		b := body(upcoming, &auth.Claims{UserID: bystander.ID, Role: model.RoleUser})
		// The key is present but null: withheld, not empty. The client distinguishes
		// them — an empty list renders "no organizers are listed", which on a round
		// that has them would be a claim.
		v, ok := b["judges"]
		if !ok {
			t.Fatal("judges key absent entirely; the client cannot tell withheld from " +
				"an older server")
		}
		if v != nil {
			t.Errorf("judges = %v on an unstarted round, want nil — naming the judges "+
				"before the start is the inference accessNotStarted prevents", v)
		}
	})

	t.Run("staff still see the roster on their own unstarted round", func(t *testing.T) {
		// The contrast that keeps the rule above honest. A setter has to be able to
		// check who they gave judge access to before the round opens, and their
		// decision is accessGranted from the staff branch, so the upcoming rule never
		// applies to them.
		b := body(upcoming, &auth.Claims{UserID: owner.ID, Role: model.RoleSetter})
		got := names(b["judges"])
		if !testListHas(got, judge.Username) {
			t.Errorf("judges = %v, want it to include %s — staff must see their own "+
				"round's roster before it starts", got, judge.Username)
		}
	})

	t.Run("a locked round refuses before naming anyone", func(t *testing.T) {
		// Anonymous on a password round. A 403 body has no roster in it by
		// construction, which is the strongest form of this assertion.
		req := httptest.NewRequest(http.MethodGet, "/api/contests/"+live.ID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", live.ID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		h.GetByID(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("GetByID = %d, want 403 — a password round must refuse an "+
				"anonymous caller rather than serve a shell with its roster", rec.Code)
		}
		if strings.Contains(rec.Body.String(), judge.Username) {
			t.Fatalf("the refusal body names the judge: %q", rec.Body.String())
		}
	})
}

func testListHas(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
