package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// ratingFixture is an in-memory contest store and rating ledger.
//
// The ledger is the interesting half: it reproduces the two rules the real store
// enforces — a row is in force only while superseded_by is empty, and a payout
// rebuilds the profiles of everyone it touched — so a test can assert what a
// dispute did to a rating rather than only what the response said.
type ratingFixture struct {
	store.ContestStore
	store.RatingStore

	contests map[string]*model.Contest
	order    []string
	problems map[string][]model.ContestProblem
	rows     map[string][]model.ScoreboardRow
	names    map[string]string
	profiles map[string]*model.UserProfile

	ledger     []model.RatingHistory
	runs       []model.RatingRun
	exclusions []model.RatingExclusion
	seq        int64
}

// teamsStub registers no teams, so the team recalculation walks an empty list. A
// round with team entries is the same code path with members to average.
type teamsStub struct {
	store.TeamStore
}

func (teamsStub) ListTeamRegistrations(_ context.Context, _ string) ([]model.TeamRegistration, error) {
	return nil, nil
}

var ratingRoundStart = time.Date(2026, 11, 14, 14, 45, 0, 0, time.UTC)

func newRatingFixture() *ratingFixture {
	return &ratingFixture{
		contests: map[string]*model.Contest{},
		problems: map[string][]model.ContestProblem{},
		rows:     map[string][]model.ScoreboardRow{},
		names:    map[string]string{},
		profiles: map[string]*model.UserProfile{},
	}
}

// addRound registers a published, rated round. board is userID → rank, where the
// rank is produced by solving the round's single problem faster or slower.
func (f *ratingFixture) addRound(id string, start time.Time, board map[string]int, ratedForMax int) {
	contest := &model.Contest{
		ID:           id,
		Status:       model.ContestStatusPublished,
		IsRated:      true,
		RatedForMin:  0,
		RatedForMax:  ratedForMax,
		Format:       "acm",
		FormatConfig: json.RawMessage(`{}`),
		StartTime:    start,
		EndTime:      start.Add(150 * time.Minute),
	}
	f.contests[id] = contest
	f.order = append(f.order, id)
	f.problems[id] = []model.ContestProblem{{ProblemID: id + "-pA", Index: "A", Score: 100}}

	for userID, rank := range board {
		f.names[userID] = userID
		if _, ok := f.profiles[userID]; !ok {
			f.profiles[userID] = &model.UserProfile{UserID: userID}
		}
		// Rank 1 solves in a minute, rank 2 in two, and so on: the ACM board sorts
		// by solved count then penalty, so this reproduces the intended order
		// without the test having to describe penalties.
		f.rows[id] = append(f.rows[id], model.ScoreboardRow{
			UserID:    userID,
			ProblemID: id + "-pA",
			Status:    "ac",
			Score:     1,
			CreatedAt: start.Add(time.Duration(rank) * time.Minute),
		})
	}
}

func (f *ratingFixture) handler() *ContestHandler {
	return &ContestHandler{
		store:       f,
		ratingStore: f,
		userStore:   fixtureUsers{profiles: f.profiles},
		teamStore:   teamsStub{},
		cache:       newScoreboardCache(time.Minute),
	}
}

// fixtureUsers is the user-store surface the contest handler performs. Separate
// from the fixture because the handler's user and contest stores both have a
// GetByID and they return different things.
type fixtureUsers struct {
	profiles map[string]*model.UserProfile
}

func (u fixtureUsers) GetByID(_ context.Context, _ string) (*model.User, error) { return nil, nil }

// GetByUsername resolves a handle the fixture knows, so an exclusion body can name
// a user the way an operator would rather than by uuid.
func (u fixtureUsers) GetByUsername(_ context.Context, username string) (*model.User, error) {
	if _, ok := u.profiles[username]; !ok {
		return nil, nil
	}
	return &model.User{ID: username, Username: username}, nil
}

func (u fixtureUsers) GetProfile(_ context.Context, userID string) (*model.UserProfile, error) {
	return u.profiles[userID], nil
}

// ── store.ContestStore ───────────────────────────────────────────────────────

func (f *ratingFixture) GetByID(_ context.Context, id string) (*model.Contest, error) {
	return f.contests[id], nil
}

func (f *ratingFixture) GetProblems(_ context.Context, contestID string) ([]model.ContestProblem, error) {
	return f.problems[contestID], nil
}

func (f *ratingFixture) GetScoreboardRows(_ context.Context, contestID string, _ *time.Time) ([]model.ScoreboardRow, error) {
	return f.rows[contestID], nil
}

func (f *ratingFixture) GetParticipants(_ context.Context, contestID string) ([]string, error) {
	// Deliberately not in finishing order, which is what the store returns:
	// GetParticipants is `SELECT DISTINCT user_id` with no ORDER BY.
	seen := map[string]bool{}
	var out []string
	for _, row := range f.rows[contestID] {
		if !seen[row.UserID] {
			seen[row.UserID] = true
			out = append(out, row.UserID)
		}
	}
	return out, nil
}

func (f *ratingFixture) GetUsernames(_ context.Context, userIDs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range userIDs {
		out[id] = f.names[id]
	}
	return out, nil
}

func (f *ratingFixture) GetDisplayNames(_ context.Context, _ string, _ []string) (map[string]model.ContestDisplayName, error) {
	return nil, nil
}

func (f *ratingFixture) ListTeamRegistrations(_ context.Context, _ string) ([]model.TeamRegistration, error) {
	return nil, nil
}

func (f *ratingFixture) RatedContestsFrom(_ context.Context, contestID string) ([]string, error) {
	var out []string
	started := false
	for _, id := range f.order {
		if id == contestID {
			started = true
		}
		if started && f.hasHistory(id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func (f *ratingFixture) HasRatingHistory(_ context.Context, contestID string) (bool, error) {
	return f.hasHistory(contestID), nil
}

func (f *ratingFixture) Delete(_ context.Context, contestID string) error {
	delete(f.contests, contestID)
	return nil
}

func (f *ratingFixture) hasHistory(contestID string) bool {
	for _, row := range f.ledger {
		if row.ContestID == contestID {
			return true
		}
	}
	return false
}

// ── store.RatingStore ────────────────────────────────────────────────────────

func (f *ratingFixture) GetByContest(_ context.Context, contestID string) ([]model.RatingHistory, error) {
	out := []model.RatingHistory{}
	for _, row := range f.ledger {
		if row.ContestID == contestID && row.SupersededBy == "" {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *ratingFixture) GetByUser(_ context.Context, userID string, _ int) ([]model.RatingHistory, error) {
	out := []model.RatingHistory{}
	for _, row := range f.ledger {
		if row.UserID == userID && row.SupersededBy == "" {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *ratingFixture) GetLatestByUser(_ context.Context, userID string) (*model.RatingHistory, error) {
	var latest *model.RatingHistory
	for i := range f.ledger {
		row := &f.ledger[i]
		if row.UserID == userID && row.SupersededBy == "" {
			latest = row
		}
	}
	return latest, nil
}

func (f *ratingFixture) CreateHistory(_ context.Context, h *model.RatingHistory) error {
	f.seq++
	h.Seq = f.seq
	f.ledger = append(f.ledger, *h)
	return nil
}

func (f *ratingFixture) BaseRatings(_ context.Context, contestID string) (map[string]int, error) {
	out := map[string]int{}
	for _, id := range f.order {
		if id == contestID {
			break
		}
		for _, row := range f.ledger {
			if row.ContestID == id && row.SupersededBy == "" {
				out[row.UserID] = row.NewRating
			}
		}
	}
	return out, nil
}

func (f *ratingFixture) ApplyRun(_ context.Context, run *model.RatingRun, scope []string, rows []model.RatingHistory) error {
	if run.ID == "" {
		run.ID = "run-" + strings.Join(scope, "-")
	}
	f.runs = append(f.runs, *run)

	inScope := map[string]bool{}
	for _, id := range scope {
		inScope[id] = true
	}
	touched := map[string]bool{}
	for i := range f.ledger {
		if f.ledger[i].SupersededBy == "" && inScope[f.ledger[i].ContestID] {
			f.ledger[i].SupersededBy = run.ID
			touched[f.ledger[i].UserID] = true
		}
	}
	for _, row := range rows {
		f.seq++
		row.RunID = run.ID
		row.Seq = f.seq
		f.ledger = append(f.ledger, row)
		touched[row.UserID] = true
	}

	f.rebuildProfiles(touched)
	return nil
}

// rebuildProfiles reproduces the store's single UPDATE: rating from the last row
// in force, max_rating and contest_count from all rows in force.
func (f *ratingFixture) rebuildProfiles(touched map[string]bool) {
	for userID := range touched {
		profile, ok := f.profiles[userID]
		if !ok {
			continue
		}
		profile.Rating, profile.MaxRating, profile.ContestCount = 0, 0, 0
		for _, id := range f.order {
			for _, row := range f.ledger {
				if row.UserID != userID || row.ContestID != id || row.SupersededBy != "" {
					continue
				}
				profile.Rating = row.NewRating
				profile.ContestCount++
				if row.NewRating > profile.MaxRating {
					profile.MaxRating = row.NewRating
				}
			}
		}
	}
}

func (f *ratingFixture) GetRunsByContest(_ context.Context, contestID string) ([]model.RatingRun, error) {
	out := []model.RatingRun{}
	for _, run := range f.runs {
		if run.ContestID == contestID {
			out = append(out, run)
		}
	}
	return out, nil
}

func (f *ratingFixture) ListExclusions(_ context.Context, contestID string) ([]model.RatingExclusion, error) {
	out := []model.RatingExclusion{}
	for _, e := range f.exclusions {
		if e.ContestID == contestID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *ratingFixture) AddExclusion(_ context.Context, e *model.RatingExclusion) error {
	for i := range f.exclusions {
		if f.exclusions[i].UserID == e.UserID && f.exclusions[i].ContestID == e.ContestID {
			f.exclusions[i].Reason = e.Reason
			return nil
		}
	}
	f.exclusions = append(f.exclusions, *e)
	return nil
}

func (f *ratingFixture) RemoveExclusion(_ context.Context, contestID, userID string) error {
	kept := f.exclusions[:0]
	for _, e := range f.exclusions {
		if e.ContestID == contestID && e.UserID == userID {
			continue
		}
		kept = append(kept, e)
	}
	f.exclusions = kept
	return nil
}

func (f *ratingFixture) SetContestVoided(_ context.Context, contestID string, voided bool) error {
	if c, ok := f.contests[contestID]; ok {
		c.RatingVoided = voided
	}
	return nil
}

// ── request helpers ──────────────────────────────────────────────────────────

func ratingRequest(method, contestID, body string, role string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, "/api/contests/"+contestID+"/ratings", nil)
	} else {
		req = httptest.NewRequest(method, "/api/contests/"+contestID+"/ratings", strings.NewReader(body))
	}
	req = withURLParam(req, "id", contestID)
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "admin1", Username: "admin", Role: role}))
}

func (f *ratingFixture) apply(t *testing.T, contestID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	f.handler().CalculateRatings(rec, ratingRequest("POST", contestID, "", "admin"))
	return rec
}

func (f *ratingFixture) recalculate(t *testing.T, contestID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	f.handler().RecalculateRatings(rec, ratingRequest("POST", contestID, "", "admin"))
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return body
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

// ── tests ────────────────────────────────────────────────────────────────────

// The bug this pins: ranks used to be `i + 1` over GetParticipants, so the
// participant the store listed first won the round no matter how they finished.
func TestCalculateRatingsRanksByBoardNotSubmissionOrder(t *testing.T) {
	f := newRatingFixture()
	// addRound inserts rows in map order, so GetParticipants order is not the
	// finishing order; ranks come from the board's penalty sort.
	f.addRound("c1", ratingRoundStart, map[string]int{"u_best": 1, "u_mid": 2, "u_low": 3}, 9999)

	rec := f.apply(t, "c1")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	changes := changesByUser(t, decodeBody(t, rec))
	if got := changes["u_best"].Rank; got != 1 {
		t.Errorf("u_best rank = %d, want 1", got)
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
// board. The entrants outside the window must not be rated, must not count toward
// the field size, and must not leave holes in the ranks.
func TestCalculateRatingsRatesOnlyTheWindow(t *testing.T) {
	f := newRatingFixture()
	f.addRound("c1", ratingRoundStart, map[string]int{"u_best": 1, "u_mid": 2, "u_low": 3}, 1599)
	// u_best is already above the window, so this round does not count for them.
	f.profiles["u_best"] = &model.UserProfile{UserID: "u_best", Rating: 1800}
	if err := f.CreateHistory(context.Background(), &model.RatingHistory{
		UserID: "u_best", ContestID: "c0", RunID: "seed", OldRating: 1700, NewRating: 1800,
	}); err != nil {
		t.Fatal(err)
	}
	f.order = append([]string{"c0"}, f.order...)

	rec := f.apply(t, "c1")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	changes := changesByUser(t, body)
	if _, rated := changes["u_best"]; rated {
		t.Error("a 1800 entrant must not be rated by a 0-1599 round")
	}
	if got := changes["u_mid"].Rank; got != 1 {
		t.Errorf("u_mid rank = %d, want 1 after the excluded winner is dropped", got)
	}
	if got := body["rated"]; got != float64(2) {
		t.Errorf("rated = %v, want 2", got)
	}
}

// A first round has no history to read, and the store reports that as 0. The
// change has to be computed against the starting rating, not against zero.
func TestCalculateRatingsStartsNewcomersAtDefaultRating(t *testing.T) {
	f := newRatingFixture()
	f.addRound("c1", ratingRoundStart, map[string]int{"u_best": 1, "u_low": 2}, 9999)

	rec := f.apply(t, "c1")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	first := changesByUser(t, decodeBody(t, rec))["u_best"]
	if first.OldRating != 1500 {
		t.Errorf("newcomer old_rating = %d, want 1500", first.OldRating)
	}
	if first.OldRating+first.RatingChange != first.NewRating {
		t.Errorf("change %+d does not take %d to %d", first.RatingChange, first.OldRating, first.NewRating)
	}
	if first.RatingChange >= 1500 {
		t.Errorf("newcomer delta = %d, which is a climb from zero rather than a first rating", first.RatingChange)
	}
}

// A second apply is a second payout, not a recalculation.
func TestCalculateRatingsRefusesSecondApply(t *testing.T) {
	f := newRatingFixture()
	f.addRound("c1", ratingRoundStart, map[string]int{"u_a": 1, "u_b": 2}, 9999)
	if rec := f.apply(t, "c1"); rec.Code != http.StatusOK {
		t.Fatalf("first apply: %d body=%s", rec.Code, rec.Body.String())
	}

	rec := f.apply(t, "c1")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(f.ledger) != 2 {
		t.Fatalf("ledger has %d rows after a refused second apply, want 2", len(f.ledger))
	}
}

// Withdrawing a round takes its effect away and leaves the ledger readable: the
// superseded rows stay, the profiles go back to what they were, and the round
// itself is still published.
func TestVoidRatingsReversesTheRoundAndRebuildsProfiles(t *testing.T) {
	f := newRatingFixture()
	f.addRound("c1", ratingRoundStart, map[string]int{"u_a": 1, "u_b": 2}, 9999)
	if rec := f.apply(t, "c1"); rec.Code != http.StatusOK {
		t.Fatalf("apply: %d body=%s", rec.Code, rec.Body.String())
	}
	afterApply := f.profiles["u_a"].Rating
	if afterApply == 0 {
		t.Fatal("apply did not move the profile")
	}

	rec := httptest.NewRecorder()
	f.handler().VoidRatings(rec, ratingRequest("POST", "c1", "", "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("void: %d body=%s", rec.Code, rec.Body.String())
	}

	if !f.contests["c1"].RatingVoided {
		t.Error("the round is not marked voided")
	}
	if got := f.profiles["u_a"].Rating; got != 0 {
		t.Errorf("rating after void = %d, want 0 (the round no longer counts)", got)
	}
	if got := f.profiles["u_a"].ContestCount; got != 0 {
		t.Errorf("contest_count after void = %d, want 0", got)
	}
	if got := f.profiles["u_a"].MaxRating; got != 0 {
		t.Errorf("max_rating after void = %d, want 0; a withdrawn peak must not stay inflated", got)
	}

	live := 0
	for _, row := range f.ledger {
		if row.SupersededBy == "" {
			live++
		}
	}
	if live != 0 {
		t.Errorf("%d rows still in force after the round was voided, want 0", live)
	}
	if len(f.ledger) != 2 {
		t.Errorf("ledger has %d rows, want the 2 superseded ones kept for the record", len(f.ledger))
	}

	// Reinstating it must land back on the original numbers, not on a second delta
	// computed from the voided state.
	rec = httptest.NewRecorder()
	f.handler().UnvoidRatings(rec, ratingRequest("DELETE", "c1", "", "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("unvoid: %d body=%s", rec.Code, rec.Body.String())
	}
	if got := f.profiles["u_a"].Rating; got != afterApply {
		t.Errorf("rating after unvoid = %d, want %d", got, afterApply)
	}
}

// A disqualification removes one entrant from the field, which changes the field
// size and everyone else's rank — so it cannot be a single-row edit.
func TestRatingExclusionShrinksTheField(t *testing.T) {
	f := newRatingFixture()
	f.addRound("c1", ratingRoundStart, map[string]int{"u_cheat": 1, "u_b": 2, "u_c": 3}, 9999)
	if rec := f.apply(t, "c1"); rec.Code != http.StatusOK {
		t.Fatalf("apply: %d body=%s", rec.Code, rec.Body.String())
	}
	beforeB := f.profiles["u_b"].Rating

	rec := httptest.NewRecorder()
	f.handler().AddRatingExclusion(rec, ratingRequest("POST", "c1", `{"user_id":"u_cheat","reason":"plagiarism"}`, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("exclude: %d body=%s", rec.Code, rec.Body.String())
	}

	if _, rated := f.profiles["u_cheat"]; !rated {
		t.Fatal("fixture lost the excluded profile")
	}
	if got := f.profiles["u_cheat"].Rating; got != 0 {
		t.Errorf("excluded entrant rating = %d, want 0", got)
	}
	if got := f.profiles["u_b"].Rating; got == beforeB {
		t.Errorf("u_b rating unchanged at %d; removing the winner must re-rank the rest", got)
	}

	changes := changesByUser(t, decodeBody(t, rec))
	if got := changes["u_b"].Rank; got != 1 {
		t.Errorf("u_b rank after exclusion = %d, want 1", got)
	}
	if _, present := changes["u_cheat"]; present {
		t.Error("the excluded entrant still appears in the field")
	}
}

// A recalculated suffix must keep the following rounds consistent: round 2 was
// computed on top of round 1, so voiding round 1 has to move round 2's base.
func TestRecalculateReplaysTheSuffix(t *testing.T) {
	f := newRatingFixture()
	// Four entrants, because the calculator barely moves a rating in a two-person
	// field and "unchanged" would be indistinguishable from "not replayed".
	board := map[string]int{"u_a": 1, "u_b": 2, "u_c": 3, "u_d": 4}
	f.addRound("c1", ratingRoundStart, board, 9999)
	f.addRound("c2", ratingRoundStart.Add(24*time.Hour), board, 9999)
	if rec := f.apply(t, "c1"); rec.Code != http.StatusOK {
		t.Fatalf("apply c1: %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.apply(t, "c2"); rec.Code != http.StatusOK {
		t.Fatalf("apply c2: %d body=%s", rec.Code, rec.Body.String())
	}
	twoRounds := f.profiles["u_a"].Rating

	rec := httptest.NewRecorder()
	f.handler().VoidRatings(rec, ratingRequest("POST", "c1", "", "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("void c1: %d body=%s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	if got := body["contests"]; got != float64(2) {
		t.Errorf("replayed %v contests, want 2 (the voided round and the one after it)", got)
	}
	// Round 2 still counts, so the rating is what round 2 alone would have produced
	// — not the two-round figure, and not zero.
	if got := f.profiles["u_a"].Rating; got == twoRounds {
		t.Errorf("rating still %d after round 1 was voided; the suffix was not replayed", got)
	}
	if got := f.profiles["u_a"].ContestCount; got != 1 {
		t.Errorf("contest_count = %d, want 1 (only round 2 still counts)", got)
	}

	// Round 2's live row must now be based on the starting rating, not on the
	// withdrawn round's result.
	live, err := f.GetByContest(context.Background(), "c2")
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 4 {
		t.Fatalf("round 2 has %d live rows, want 4", len(live))
	}
	if live[0].OldRating != 1500 {
		t.Errorf("round 2 base = %d, want 1500: it must be replayed from the round before the voided one", live[0].OldRating)
	}
}

// The rating endpoints are admin-only, and the ledger is not touched by a refusal.
func TestRatingEndpointsRefuseNonAdmins(t *testing.T) {
	f := newRatingFixture()
	f.addRound("c1", ratingRoundStart, map[string]int{"u_a": 1, "u_b": 2}, 9999)

	rec := httptest.NewRecorder()
	f.handler().CalculateRatings(rec, ratingRequest("POST", "c1", "", "user"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if len(f.ledger) != 0 {
		t.Fatalf("a refused request wrote %d ledger rows", len(f.ledger))
	}
}

// Deleting a round that owns rating history would take the ledger with it.
func TestDeleteRefusesContestWithRatingHistory(t *testing.T) {
	f := newRatingFixture()
	f.addRound("c1", ratingRoundStart, map[string]int{"u_a": 1, "u_b": 2}, 9999)
	if rec := f.apply(t, "c1"); rec.Code != http.StatusOK {
		t.Fatalf("apply: %d body=%s", rec.Code, rec.Body.String())
	}

	rec := httptest.NewRecorder()
	f.handler().Delete(rec, ratingRequest("DELETE", "c1", "", "admin"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := f.contests["c1"]; !ok {
		t.Fatal("the round was deleted despite owning rating history")
	}
}
