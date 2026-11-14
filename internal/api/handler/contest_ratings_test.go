package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// The rating job reads its order from the standings board, so these stubs hand
// the handler a participant list in an order that is NOT the finishing order —
// which is what the store returns, since GetParticipants is `SELECT DISTINCT
// user_id` with no ORDER BY.

type ratingsStub struct {
	store.RatingStore
	applied []model.RatingHistory
	latest  map[string]*model.RatingHistory
	created []model.RatingHistory
}

func (s *ratingsStub) GetByContest(_ context.Context, _ string) ([]model.RatingHistory, error) {
	return s.applied, nil
}

func (s *ratingsStub) GetLatestByUser(_ context.Context, userID string) (*model.RatingHistory, error) {
	return s.latest[userID], nil
}

func (s *ratingsStub) CreateHistory(_ context.Context, h *model.RatingHistory) error {
	s.created = append(s.created, *h)
	return nil
}

type usersStub struct {
	profiles map[string]*model.UserProfile
	updated  map[string]int
}

func (s *usersStub) GetByID(_ context.Context, _ string) (*model.User, error) { return nil, nil }

func (s *usersStub) GetByUsername(_ context.Context, _ string) (*model.User, error) { return nil, nil }

func (s *usersStub) GetProfile(_ context.Context, userID string) (*model.UserProfile, error) {
	return s.profiles[userID], nil
}

func (s *usersStub) UpdateRating(_ context.Context, userID string, rating, _, _ int) error {
	if s.updated == nil {
		s.updated = map[string]int{}
	}
	s.updated[userID] = rating
	return nil
}

type contestBoardStub struct {
	store.ContestStore
	contest      *model.Contest
	problems     []model.ContestProblem
	rows         []model.ScoreboardRow
	participants []string
	usernames    map[string]string
}

func (s contestBoardStub) GetByID(_ context.Context, _ string) (*model.Contest, error) {
	return s.contest, nil
}

func (s contestBoardStub) GetProblems(_ context.Context, _ string) ([]model.ContestProblem, error) {
	return s.problems, nil
}

func (s contestBoardStub) GetScoreboardRows(_ context.Context, _ string, _ *time.Time) ([]model.ScoreboardRow, error) {
	return s.rows, nil
}

func (s contestBoardStub) GetParticipants(_ context.Context, _ string) ([]string, error) {
	return s.participants, nil
}

func (s contestBoardStub) GetUsernames(_ context.Context, _ []string) (map[string]string, error) {
	return s.usernames, nil
}

func (s contestBoardStub) GetDisplayNames(_ context.Context, _ string, _ []string) (map[string]model.ContestDisplayName, error) {
	return nil, nil
}

// The handler passes the contest store as the team registrar, so this round has
// to answer the registration listing even though it registers no teams.
func (s contestBoardStub) ListTeamRegistrations(_ context.Context, _ string) ([]model.TeamRegistration, error) {
	return nil, nil
}

type teamsStub struct {
	store.TeamStore
}

func (s teamsStub) ListTeamRegistrations(_ context.Context, _ string) ([]model.TeamRegistration, error) {
	return nil, nil
}

var contestStart = time.Date(2026, 11, 14, 14, 45, 0, 0, time.UTC)

// rateContest drives the admin rating endpoint over a three-problem-field round
// where u_best finishes first, u_mid second and u_low last, while the store
// lists them in the reverse order.
func rateContest(t *testing.T, ratedForMax int, ratings *ratingsStub) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()

	contest := &model.Contest{
		ID:           "c1",
		Status:       model.ContestStatusPublished,
		IsRated:      true,
		RatedForMin:  0,
		RatedForMax:  ratedForMax,
		Format:       "acm",
		FormatConfig: json.RawMessage(`{}`),
		StartTime:    contestStart,
		EndTime:      contestStart.Add(150 * time.Minute),
	}

	board := contestBoardStub{
		contest:  contest,
		problems: []model.ContestProblem{{ProblemID: "pA", Index: "A", Score: 100}},
		rows: []model.ScoreboardRow{
			{UserID: "u_best", ProblemID: "pA", Status: "ac", CreatedAt: contestStart.Add(10 * time.Minute)},
			{UserID: "u_mid", ProblemID: "pA", Status: "ac", CreatedAt: contestStart.Add(30 * time.Minute)},
			{UserID: "u_low", ProblemID: "pA", Status: "wa", CreatedAt: contestStart.Add(5 * time.Minute)},
		},
		// The order Postgres happened to return, and the reason the rating job
		// used to hand first place to whoever was written first.
		participants: []string{"u_low", "u_mid", "u_best"},
		usernames: map[string]string{
			"u_best": "best", "u_mid": "mid", "u_low": "low",
		},
	}

	h := &ContestHandler{
		store:       board,
		ratingStore: ratings,
		userStore: &usersStub{profiles: map[string]*model.UserProfile{
			"u_best": {Rating: 1200},
			"u_mid":  {Rating: 1300},
			"u_low":  {Rating: 1400},
		}},
		teamStore: teamsStub{},
		cache:     newScoreboardCache(time.Minute),
	}

	req := httptest.NewRequest("POST", "/api/contests/c1/calculate-ratings", nil)
	req = withURLParam(req, "id", "c1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "admin1", Role: "admin"}))
	rec := httptest.NewRecorder()
	h.CalculateRatings(rec, req)

	body := map[string]interface{}{}
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
		}
	}
	return rec, body
}

func changesByUser(t *testing.T, body map[string]interface{}) map[string]model.RatingChange {
	t.Helper()
	raw, _ := json.Marshal(body["changes"])
	var changes []model.RatingChange
	if err := json.Unmarshal(raw, &changes); err != nil {
		t.Fatalf("decode changes: %v", err)
	}
	out := make(map[string]model.RatingChange, len(changes))
	for _, c := range changes {
		out[c.UserID] = c
	}
	return out
}

// The bug this pins: ranks used to be `i + 1` over GetParticipants, so the
// participant the store listed first won the round no matter how they finished.
func TestCalculateRatingsRanksByBoardNotSubmissionOrder(t *testing.T) {
	rec, body := rateContest(t, 9999, &ratingsStub{})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	changes := changesByUser(t, body)
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	if got := changes["u_best"].Rank; got != 1 {
		t.Errorf("u_best rank = %d, want 1", got)
	}
	if got := changes["u_mid"].Rank; got != 2 {
		t.Errorf("u_mid rank = %d, want 2", got)
	}
	if got := changes["u_low"].Rank; got != 3 {
		t.Errorf("u_low rank = %d, want 3", got)
	}

	if changes["u_best"].RatingChange <= 0 {
		t.Errorf("first place should gain rating, got %+d", changes["u_best"].RatingChange)
	}
	if changes["u_low"].RatingChange >= 0 {
		t.Errorf("last place should lose rating, got %+d", changes["u_low"].RatingChange)
	}
}

// A round rated for a rating window still carries everyone who submitted on its
// board. The entrants outside the window must not be rated, must not count
// toward the field size, and must not leave holes in the ranks.
func TestCalculateRatingsRatesOnlyTheWindow(t *testing.T) {
	ratings := &ratingsStub{latest: map[string]*model.RatingHistory{
		"u_best": {NewRating: 1800},
	}}
	rec, body := rateContest(t, 1599, ratings)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	changes := changesByUser(t, body)
	if _, rated := changes["u_best"]; rated {
		t.Error("a 1800 entrant must not be rated by a 0-1599 round")
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 rated entrants, got %d", len(changes))
	}
	if got := changes["u_mid"].Rank; got != 1 {
		t.Errorf("u_mid rank = %d, want 1 after the excluded winner is dropped", got)
	}
	if got := changes["u_low"].Rank; got != 2 {
		t.Errorf("u_low rank = %d, want 2", got)
	}
	if got := body["rated"]; got != float64(2) {
		t.Errorf("rated = %v, want 2", got)
	}
	if got := body["participants"]; got != float64(3) {
		t.Errorf("participants = %v, want 3", got)
	}
}

// A first round has no history to read, and the store reports that as 0. The
// change has to be computed against the starting rating, not against zero — the
// graph otherwise shows a newcomer climbing from nothing to 1500.
func TestCalculateRatingsStartsNewcomersAtDefaultRating(t *testing.T) {
	ratings := &ratingsStub{}
	rec, body := rateContest(t, 9999, ratings)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	first := changesByUser(t, body)["u_best"]
	if first.OldRating != 1500 {
		t.Errorf("newcomer old_rating = %d, want 1500", first.OldRating)
	}
	if first.OldRating+first.RatingChange != first.NewRating {
		t.Errorf("change %+d does not take %d to %d", first.RatingChange, first.OldRating, first.NewRating)
	}
	if first.RatingChange >= 1500 {
		t.Errorf("newcomer delta = %d, which is a climb from zero rather than a first rating", first.RatingChange)
	}

	for _, h := range ratings.created {
		if h.UserID == "u_best" && h.OldRating != 1500 {
			t.Errorf("recorded history old_rating = %d, want 1500", h.OldRating)
		}
	}
}

// Ratings are irreversible, so the second click must not pay out twice.
func TestCalculateRatingsRefusesSecondRun(t *testing.T) {
	ratings := &ratingsStub{applied: []model.RatingHistory{{UserID: "u_best", ContestID: "c1"}}}
	rec, _ := rateContest(t, 9999, ratings)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(ratings.created) != 0 {
		t.Fatalf("second run wrote %d history rows, want 0", len(ratings.created))
	}
}
