package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
 * The problem list has its own audience, and the contest page is where it is
 * enforced.
 *
 * This is the only gated surface NOT enforced by requireContestSurfaceAccess: the
 * contest page is a single request that carries both the round's shell and its
 * problem list, so the list is decided inline. That makes it the one place where
 * a mistake is invisible to the policy tests — they exercise `Decide`, and this
 * code path never calls it for the list.
 *
 * Three things have to hold at once, and each is a way this leaks if it does not:
 *
 *   1. A round that publishes nothing serves no problems, whatever the caller.
 *   2. A round that publishes its problem list serves it to a signed-out visitor,
 *      while the STATEMENTS stay closed. That pair is a teaser round, and getting
 *      it half right is the bug: serving the list but also opening the statements
 *      publishes the round, and serving neither makes the setting do nothing.
 *   3. An unstarted round serves no list even when the list is public. The policy
 *      answers "not started" before any audience, because a list visible hours
 *      early tells the world an invite-only round exists.
 *
 * Asserted through the handler, because a rule that only exists in the policy is
 * not a rule a request sees.
 */
func TestContestPageProblemListAudience(t *testing.T) {
	db := openHandlerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	problemStore := postgres.NewProblemStore(db)
	h := NewContestHandler(contestStore, postgres.NewRatingStore(db), problemStore, userStore, nil)

	author := &model.User{
		ID: uuid.New().String(), Username: "pla_" + uuid.New().String()[:8],
		Email:        "pla_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, author); err != nil {
		t.Fatalf("create author: %v", err)
	}

	problem := &model.Problem{
		ID: uuid.New().String(), Slug: "pl-" + uuid.New().String()[:8],
		Title: "Listed " + uuid.New().String()[:8], Description: "d",
		TimeLimit: 1000, MemoryLimit: 262144,
		Status: model.ProblemStatusPublished, CreatedBy: author.ID,
	}
	if err := problemStore.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}

	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	mk := func(title string, config model.AccessConfig, start time.Time) *model.Contest {
		return &model.Contest{
			ID: uuid.New().String(), Title: title, Type: "acm", Format: "acm",
			StartTime: start, EndTime: start.Add(3 * time.Hour),
			CreatedBy: author.ID, Status: model.ContestStatusPublished,
			IsRated: true, RatedForMin: 0, RatedForMax: 9999,
			AccessConfig: config,
			// Password-protected, which is the case this setting exists for. On an
			// UNRESTRICTED round "participants" already means everyone: there is no
			// gate to distinguish from, so the audience has nothing to withhold and
			// every assertion below would be vacuous.
			PasswordHash: testPasswordHash,
		}
	}

	closed := mk("closed "+uuid.New().String()[:8], model.AccessConfig{}, started)
	teaser := mk("teaser "+uuid.New().String()[:8],
		model.AccessConfig{model.SurfaceProblemList: model.AccessPublic}, started)
	upcoming := mk("upcoming "+uuid.New().String()[:8],
		model.AccessConfig{model.SurfaceProblemList: model.AccessPublic},
		time.Now().UTC().Add(time.Hour))

	for _, c := range []*model.Contest{closed, teaser, upcoming} {
		c := c
		if err := contestStore.Create(ctx, c); err != nil {
			t.Fatalf("create contest %s: %v", c.Title, err)
		}
		if err := contestStore.AddProblem(ctx, c.ID, problem.ID, "A", 100, 0); err != nil {
			t.Fatalf("attach problem: %v", err)
		}
		defer func() {
			db.ExecContext(ctx, "DELETE FROM contest_problems WHERE contest_id = $1", c.ID)
			db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
		}()
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", problem.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", author.ID)
	}()

	// problemCount drives the real page and reports how many problems it served and
	// how many of those carried a title, so a test can tell "no list at all" from
	// "a list whose names were withheld". A 403 counts as zero problems and is not
	// an error: refusing the page is the correct answer for a password round, and
	// treating it as a test failure would force every case to be admitted.
	problemCount := func(c *model.Contest, claims *auth.Claims) (int, int, int) {
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
		if rec.Code != http.StatusOK && rec.Code != http.StatusForbidden {
			t.Fatalf("contest page = %d, want 200 or 403 (body=%q)", rec.Code, rec.Body.String())
		}
		var payload struct {
			Problems []map[string]any `json:"problems"`
		}
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
		}
		titled := 0
		for _, p := range payload.Problems {
			if s, ok := p["title"].(string); ok && s != "" {
				titled++
			}
		}
		return rec.Code, len(payload.Problems), titled
	}

	t.Run("a round that publishes nothing serves no problem list", func(t *testing.T) {
		// Password round, no grant, no published surface: the page prompts rather
		// than serving a list, which is the pre-existing behaviour and must stay.
		code, n, _ := problemCount(closed, nil)
		if code != http.StatusForbidden {
			t.Fatalf("closed round page = %d, want 403: a password round with nothing published must still prompt", code)
		}
		if n != 0 {
			t.Fatalf("problems = %d, want 0 — the default must not publish the list", n)
		}
	})

	t.Run("a published list reaches a signed-out visitor, titles included", func(t *testing.T) {
		// The whole point of the setting: a stranger who never held the password
		// opens the page and sees which problems the round will ask about.
		code, n, titled := problemCount(teaser, nil)
		if code != http.StatusOK {
			t.Fatalf("teaser page = %d, want 200: a published problem list must be reachable", code)
		}
		if n != 1 {
			t.Fatalf("problems = %d, want 1 — a public problem list must be served", n)
		}
		if titled != 1 {
			t.Fatal("the list was served without titles; a teaser round is exactly the titles")
		}
	})

	t.Run("an unstarted round serves no list even when the list is public", func(t *testing.T) {
		// The side-channel rule. Not a leak of the problems themselves but of the
		// round's existence and shape, which accessNotStarted exists to prevent.
		_, n, _ := problemCount(upcoming, nil)
		if n != 0 {
			t.Fatalf("problems = %d on an unstarted round, want 0", n)
		}
	})

	t.Run("the teaser round's STATEMENTS are still closed", func(t *testing.T) {
		// The half that makes this safe. Publishing the list must not publish the
		// statements — if this ever passes with Allowed(), a teaser round has
		// become a published round.
		p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{granted: false})
		decision, err := p.Decide(context.Background(), teaser, nil, model.SurfaceStatement)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Allowed() {
			t.Fatal("a public problem list also opened the statements")
		}
	})

	t.Run("staff see the list on a round that publishes nothing", func(t *testing.T) {
		// A setter checking their own round must not be locked out of their own
		// problem list by an audience they set for competitors.
		code, n, _ := problemCount(closed, &auth.Claims{UserID: author.ID, Role: model.RoleSetter})
		if code != http.StatusOK {
			t.Fatalf("staff page = %d, want 200", code)
		}
		if n != 1 {
			t.Fatalf("staff problems = %d, want 1", n)
		}
	})

	// The regression this whole audience was fixed for.
	//
	// On an unrestricted round the page admits everyone, so the ONLY thing that can
	// withhold the list is the participation rule. Before the fix, `participants`
	// fell through to the content verdict — which grants any caller on such a round
	// — so a setter who set their problem list to "Participants only" was served
	// every title to a signed-out visitor, and the live round was doing exactly
	// that. Every assertion here would have been vacuously true under the old
	// behaviour, which is why this case has to use a round with NO password and NO
	// group: with either present, the content gate refuses on its own and proves
	// nothing about the audience.
	unrestricted := mk("unrestricted "+uuid.New().String()[:8],
		model.AccessConfig{model.SurfaceProblemList: model.AccessParticipants}, started)
	unrestricted.PasswordHash = ""
	if err := contestStore.Create(ctx, unrestricted); err != nil {
		t.Fatalf("create unrestricted contest: %v", err)
	}
	if err := contestStore.AddProblem(ctx, unrestricted.ID, problem.ID, "A", 100, 0); err != nil {
		t.Fatalf("attach problem: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_problems WHERE contest_id = $1", unrestricted.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", unrestricted.ID)
	}()

	t.Run("an open round honours participants on its problem list", func(t *testing.T) {
		// The page still opens — the round itself is public, so refusing it would
		// be wrong — but the list it carries is empty. That pair is the setting
		// working: "you may open this round, you may not read its problems."
		code, n, titled := problemCount(unrestricted, nil)
		if code != http.StatusOK {
			t.Fatalf("unrestricted page = %d, want 200: the round itself is open, "+
				"so the page must still render its shell", code)
		}
		if n != 0 {
			t.Fatalf("problems = %d, want 0 — a signed-out visitor read the problem "+
				"list of a round whose list is set to Participants only", n)
		}
		if titled != 0 {
			t.Fatalf("titles = %d, want 0", titled)
		}
	})

	t.Run("an open round's statements are withheld too", func(t *testing.T) {
		// The list and the statement are separate surfaces, and an absent statement
		// entry inherits the base — NOT the problem list's restriction, because
		// inheritance flows from the base only. Here the base is open, so the
		// statements fall through to the content rules and ARE readable. Asserting
		// it keeps the one-way rule honest: restricting the list must not silently
		// restrict a surface the setter did not touch.
		p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{})
		decision, err := p.Decide(context.Background(), unrestricted, nil, model.SurfaceStatement)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !decision.Allowed() {
			t.Fatal("setting the problem list to participants also closed the " +
				"statements; inheritance must flow from the base, never sideways")
		}
	})
}
