package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The ledger against a real database.
 *
 * The store's SQL is the one part of this feature that no handler test can reach:
 * the handler tests drive an in-memory ledger, so they prove the policy and prove
 * nothing about the schema. That gap shipped a 500 on both rating endpoints — the
 * store still named a column and a run kind that the migration had dropped, and
 * every test in the repo passed.
 *
 * So this test runs the real statements: a payout, the run listing, the base
 * lookup a replay starts from, the profile rebuild, and the two constraints that
 * make the ledger trustworthy.
 */

func TestRatingLedgerAgainstRealSchema(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// A database behind migration 000094 is not a broken test, it is an unmigrated
	// one — and this test's whole subject is the schema, so there is nothing to
	// assert against. Skip with the fix rather than reporting a failure that reads
	// like a code defect.
	var ledgerReady bool
	if err := db.QueryRowContext(ctx,
		`SELECT to_regclass('rating_runs') IS NOT NULL
		    AND EXISTS (SELECT 1 FROM information_schema.columns
		                 WHERE table_name = 'rating_history' AND column_name = 'superseded_by')`,
	).Scan(&ledgerReady); err != nil {
		t.Fatalf("probe schema: %v", err)
	}
	if !ledgerReady {
		t.Skip("database is not migrated to 000094; run `make migrate-up` to exercise the ledger")
	}

	ratings := NewRatingStore(db)
	contests := NewContestStore(db)
	users := NewUserStore(db)

	userA, userB := uuid.NewString(), uuid.NewString()
	for _, u := range []*model.User{
		{ID: userA, Username: "ledger_a_" + uuid.NewString()[:8], Email: userA[:8] + "@example.com", PasswordHash: "x", Role: model.RoleUser},
		{ID: userB, Username: "ledger_b_" + uuid.NewString()[:8], Email: userB[:8] + "@example.com", PasswordHash: "x", Role: model.RoleUser},
	} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	}

	now := time.Now().UTC()
	c1, c2 := uuid.NewString(), uuid.NewString()
	// Published and rated, because Create defaults to draft and the suffix listing
	// only walks rounds that could have been paid out.
	for _, c := range []*model.Contest{
		{ID: c1, Title: "ledger round 1", Type: "acm", Format: "acm", StartTime: now, EndTime: now.Add(3 * time.Hour),
			CreatedBy: userA, Status: model.ContestStatusPublished, IsRated: true, RatedForMax: 9999},
		{ID: c2, Title: "ledger round 2", Type: "acm", Format: "acm", StartTime: now.Add(24 * time.Hour), EndTime: now.Add(27 * time.Hour),
			CreatedBy: userA, Status: model.ContestStatusPublished, IsRated: true, RatedForMax: 9999},
	} {
		if err := contests.Create(ctx, c); err != nil {
			t.Fatalf("create contest: %v", err)
		}
	}
	// Rows before rounds, runs before rows: both references are RESTRICT, which is
	// the point of the migration and the reason this order is written out.
	defer func() {
		db.ExecContext(ctx, "DELETE FROM rating_history WHERE contest_id = ANY($1)", pq.Array([]string{c1, c2}))
		db.ExecContext(ctx, "DELETE FROM rating_runs WHERE contest_id = ANY($1)", pq.Array([]string{c1, c2}))
		db.ExecContext(ctx, "DELETE FROM rating_exclusions WHERE contest_id = ANY($1)", pq.Array([]string{c1, c2}))
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = ANY($1)", pq.Array([]string{c1, c2}))
	}()

	// A payout: one run, N rows, profiles rebuilt in the same transaction.
	apply := &model.RatingRun{ContestID: c1, Kind: model.RatingRunApply, ActorID: userA, ActorName: "admin"}
	if err := ratings.ApplyRun(ctx, apply, []string{c1}, []model.RatingHistory{
		{UserID: userA, ContestID: c1, OldRating: 1500, NewRating: 1600, Rank: 1, RatingChange: 100},
		{UserID: userB, ContestID: c1, OldRating: 1500, NewRating: 1400, Rank: 2, RatingChange: -100},
	}); err != nil {
		t.Fatalf("ApplyRun: %v", err)
	}

	// The run listing is what the operator's history panel reads.
	runs, err := ratings.GetRunsByContest(ctx, c1)
	if err != nil {
		t.Fatalf("GetRunsByContest: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].Kind != model.RatingRunApply || runs[0].ActorName != "admin" || runs[0].ID != apply.ID {
		t.Errorf("run = %+v, want the apply run by admin", runs[0])
	}

	live, err := ratings.GetByContest(ctx, c1)
	if err != nil {
		t.Fatalf("GetByContest: %v", err)
	}
	if len(live) != 2 {
		t.Fatalf("live rows = %d, want 2", len(live))
	}
	if live[0].RunID != apply.ID {
		t.Errorf("row run_id = %q, want %q", live[0].RunID, apply.ID)
	}

	// The base a replay starts from: round 1's result, read for round 2.
	base, err := ratings.BaseRatings(ctx, c2)
	if err != nil {
		t.Fatalf("BaseRatings: %v", err)
	}
	if base[userA] != 1600 || base[userB] != 1400 {
		t.Errorf("base = %+v, want a:1600 b:1400", base)
	}

	// A recalculation is a different kind, and the check constraint has to accept
	// it — the two disagreed once, which made every recalculation a 500.
	recalc := &model.RatingRun{ContestID: c1, Kind: model.RatingRunRecalc, Reason: "round voided"}
	if err := ratings.ApplyRun(ctx, recalc, []string{c1}, []model.RatingHistory{
		{UserID: userA, ContestID: c1, OldRating: 1500, NewRating: 1520, Rank: 1, RatingChange: 20},
		{UserID: userB, ContestID: c1, OldRating: 1500, NewRating: 1420, Rank: 2, RatingChange: -80},
	}); err != nil {
		t.Fatalf("ApplyRun(recalc): %v", err)
	}

	// Superseded, not deleted: the previous decision is still readable.
	var total, inForce int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE superseded_by IS NULL)
		   FROM rating_history WHERE contest_id = $1`, c1).Scan(&total, &inForce); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if total != 4 || inForce != 2 {
		t.Errorf("ledger has %d rows (%d in force), want 4 (2)", total, inForce)
	}

	// The profile is the cache, rebuilt from the rows in force.
	var rating, maxRating, count int
	if err := db.QueryRowContext(ctx,
		`SELECT rating, max_rating, contest_count FROM user_profiles WHERE user_id = $1`, userA).
		Scan(&rating, &maxRating, &count); err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if rating != 1520 || maxRating != 1520 || count != 1 {
		t.Errorf("profile a = rating %d max %d count %d, want 1520/1520/1", rating, maxRating, count)
	}

	// Reading one user's history orders by the round's place in the calendar.
	history, err := ratings.GetByUser(ctx, userA, 10)
	if err != nil {
		t.Fatalf("GetByUser: %v", err)
	}
	if len(history) != 1 || history[0].NewRating != 1520 {
		t.Errorf("history = %+v, want one row at 1520", history)
	}
	latest, err := ratings.GetLatestByUser(ctx, userA)
	if err != nil {
		t.Fatalf("GetLatestByUser: %v", err)
	}
	if latest == nil || latest.NewRating != 1520 {
		t.Errorf("latest = %+v, want 1520", latest)
	}

	// A second live decision for one (user, round) is refused by the index, not
	// only by the endpoint's read-then-write check.
	if err := ratings.CreateHistory(ctx, &model.RatingHistory{
		UserID: userA, ContestID: c1, RunID: recalc.ID,
		OldRating: 1, NewRating: 2, Rank: 1, RatingChange: 1,
	}); err == nil {
		t.Error("a second live row was accepted; the partial unique index is missing")
	}

	// Exclusions and the void flag round out the ledger's operations.
	if err := ratings.AddExclusion(ctx, &model.RatingExclusion{
		UserID: userB, ContestID: c1, Reason: "plagiarism", CreatedBy: userA,
	}); err != nil {
		t.Fatalf("AddExclusion: %v", err)
	}
	exclusions, err := ratings.ListExclusions(ctx, c1)
	if err != nil {
		t.Fatalf("ListExclusions: %v", err)
	}
	if len(exclusions) != 1 || exclusions[0].Reason != "plagiarism" {
		t.Errorf("exclusions = %+v", exclusions)
	}
	if err := ratings.RemoveExclusion(ctx, c1, userB); err != nil {
		t.Fatalf("RemoveExclusion: %v", err)
	}
	if err := ratings.SetContestVoided(ctx, c1, true); err != nil {
		t.Fatalf("SetContestVoided: %v", err)
	}

	// The suffix listing stops at rounds that were actually paid out.
	ids, err := contests.RatedContestsFrom(ctx, c1)
	if err != nil {
		t.Fatalf("RatedContestsFrom: %v", err)
	}
	if len(ids) != 1 || ids[0] != c1 {
		t.Errorf("RatedContestsFrom = %v, want [%s] (round 2 was never rated)", ids, c1)
	}

	// The delete guard reads this, so it has to answer for a round that owns rows.
	has, err := contests.HasRatingHistory(ctx, c1)
	if err != nil {
		t.Fatalf("HasRatingHistory: %v", err)
	}
	if !has {
		t.Error("HasRatingHistory = false for a round with ledger rows")
	}
	if has, err := contests.HasRatingHistory(ctx, c2); err != nil {
		t.Fatalf("HasRatingHistory(round 2): %v", err)
	} else if has {
		t.Error("HasRatingHistory = true for a round that was never rated")
	}

	// Deleting a rated round is refused by the database, which is what stops the
	// ledger being taken with it.
	if err := contests.Delete(ctx, c1); err == nil {
		t.Error("deleting a round that owns rating history was accepted")
	}
}
