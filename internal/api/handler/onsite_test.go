package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

type onsiteContestStoreStub struct {
	contest *model.Contest
}

func (s onsiteContestStoreStub) GetByID(_ context.Context, id string) (*model.Contest, error) {
	if id == "12" {
		return s.contest, nil
	}
	return nil, nil
}

func (onsiteContestStoreStub) HasAccess(context.Context, string, string, ...string) bool {
	return true
}

type onsiteBalloonStoreStub struct {
	contestID string
}

func (s *onsiteBalloonStoreStub) CreateRequest(context.Context, string, string, string, string) error {
	return nil
}
func (s *onsiteBalloonStoreStub) ListByContest(_ context.Context, contestID string) ([]model.BalloonRequest, error) {
	s.contestID = contestID
	return []model.BalloonRequest{}, nil
}
func (*onsiteBalloonStoreStub) Dispatch(context.Context, string) error { return nil }

func onsiteRequest(path, id string, claims *auth.Claims) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if claims != nil {
		ctx = context.WithValue(ctx, middleware.UserContextKey, claims)
	}
	return req.WithContext(ctx)
}

func TestOnsiteListBalloonsResolvesDisplayIDBeforeStoreQuery(t *testing.T) {
	contests := onsiteContestStoreStub{contest: &model.Contest{ID: "00000000-0000-0000-0000-000000000012"}}
	balloons := &onsiteBalloonStoreStub{}
	h := NewOnsiteHandler(balloons, nil, contests)

	rec := httptest.NewRecorder()
	h.ListBalloons(rec, onsiteRequest("/api/contests/12/onsite/balloons", "12", &auth.Claims{UserID: "admin", Role: "admin"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if balloons.contestID != contests.contest.ID {
		t.Fatalf("balloon store received contest %q, want canonical %q", balloons.contestID, contests.contest.ID)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestOnsiteListBalloonsReturnsNotFoundForUnknownDisplayID(t *testing.T) {
	contests := onsiteContestStoreStub{contest: &model.Contest{ID: "canonical"}}
	balloons := &onsiteBalloonStoreStub{}
	h := NewOnsiteHandler(balloons, nil, contests)

	rec := httptest.NewRecorder()
	h.ListBalloons(rec, onsiteRequest("/api/contests/999/onsite/balloons", "999", &auth.Claims{UserID: "admin", Role: "admin"}))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if balloons.contestID != "" {
		t.Fatalf("balloon store was queried for unknown contest %q", balloons.contestID)
	}
}
