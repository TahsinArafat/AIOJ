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
 * A judge's board must not be cached under a spectator's key.
 *
 * Reported from a live round: signed out, the standings page offered a "Judge
 * View" toggle. The scoreboard returns can_see_judge so the page knows whether to
 * show that toggle, and can_see_judge is a per-caller fact — but it was the one
 * per-caller field missing from the scoreboard cache key.
 *
 * The collision needs a judge to load the public view: `?view=public` sets
 * is_judge=false, so a judge's response then matches an anonymous spectator's on
 * every OTHER component of the key (same contest, same view, same page, same
 * limit, and canReadRound=true because an unrestricted round is readable by
 * anyone). The judge's entry — which carries can_see_judge=true — was served to
 * the next anonymous visitor, whose page then rendered the toggle.
 *
 * This is why X-Cache mattered while diagnosing it: the leak was a HIT, not a
 * freshly computed response, which is also why the response looked correct when
 * probed without the ?view=public parameter.
 */
func TestScoreboardCacheDoesNotSeedJudgeCapabilityToSpectators(t *testing.T) {
	db := openHandlerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	// Built through the real constructor: the handler needs its access policy
	// wired, and hand-assembling the struct leaves h.access nil and panics.
	h := NewContestHandler(contestStore, postgres.NewRatingStore(db), nil, userStore, nil)

	judge := &model.User{
		ID: uuid.New().String(), Username: "jst_" + uuid.New().String()[:8],
		Email:        "jst_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, judge); err != nil {
		t.Fatalf("create judge: %v", err)
	}
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "Cache " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(3 * time.Hour),
		CreatedBy: judge.ID, Status: model.ContestStatusPublished,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
		// A public board is what makes this reachable by a signed-out spectator,
		// and it is the setting the round in question used.
		ScoreboardVisibility: model.ScoreboardPublic,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", judge.ID)
	}()

	// 1. The judge loads the public view. This is the request that seeds the entry.
	judgeRec := getScoreboard(t, h, c.ID, "public", &auth.Claims{UserID: judge.ID, Role: model.RoleSetter})
	if judgeRec.Code != http.StatusOK {
		t.Fatalf("judge board = %d, want 200 (body=%q)", judgeRec.Code, judgeRec.Body.String())
	}
	if !scoreboardFlag(t, judgeRec.Body.Bytes(), "can_see_judge") {
		t.Fatal("the judge's own response must say can_see_judge=true, or this test proves nothing")
	}

	// 2. A signed-out visitor loads the same URL. Before the fix this was a cache
	//    HIT on the judge's entry and returned can_see_judge=true.
	anonRec := getScoreboard(t, h, c.ID, "public", nil)
	if anonRec.Code != http.StatusOK {
		t.Fatalf("anonymous board = %d, want 200 (body=%q)", anonRec.Code, anonRec.Body.String())
	}
	if scoreboardFlag(t, anonRec.Body.Bytes(), "can_see_judge") {
		t.Fatalf("a signed-out visitor was told can_see_judge=true (x-cache=%q) — the standings page renders its Judge View toggle from this field",
			anonRec.Header().Get("X-Cache"))
	}
	if scoreboardFlag(t, anonRec.Body.Bytes(), "is_judge") {
		t.Fatal("a signed-out visitor was told is_judge=true")
	}
}

func getScoreboard(t *testing.T, h *ContestHandler, contestID, view string, claims *auth.Claims) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/contests/"+contestID+"/scoreboard?view="+view, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", contestID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	rec := httptest.NewRecorder()
	h.Scoreboard(rec, req)
	return rec
}

func scoreboardFlag(t *testing.T, body []byte, key string) bool {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode scoreboard: %v", err)
	}
	v, ok := payload[key].(bool)
	if !ok {
		return false
	}
	return v
}
