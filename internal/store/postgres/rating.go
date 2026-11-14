package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

// RatingStore reads and writes the rating ledger.
//
// The ledger is append-only. A recalculation never edits or deletes a row: it
// marks the rows it replaces with superseded_by and writes replacements. So a row
// is in force exactly when superseded_by IS NULL, and "what was this rating
// before the dispute" stays answerable after the dispute is resolved.
type RatingStore struct {
	db *sql.DB
}

func NewRatingStore(db *sql.DB) *RatingStore {
	return &RatingStore{db: db}
}

// liveColumns is the ledger projection for a query that reads rating_history
// alone, or for a RETURNING clause — where Postgres refuses a table qualifier.
const liveColumns = `id, user_id, contest_id, run_id, COALESCE(superseded_by::text, ''), seq,
	                     old_rating, new_rating, rank, rating_change, created_at`

// liveColumnsFrom is the same projection qualified for a query that joins
// another table. Both rating_history and contests carry an `id`, so an
// unqualified projection is ambiguous there — which is not a compile error, a
// test error, or anything the in-memory fixtures can see: it is a 500 on the
// rating history page.
func liveColumnsFrom(alias string) string {
	return alias + `.id, ` + alias + `.user_id, ` + alias + `.contest_id, ` + alias + `.run_id, ` +
		`COALESCE(` + alias + `.superseded_by::text, ''), ` + alias + `.seq, ` +
		alias + `.old_rating, ` + alias + `.new_rating, ` + alias + `.rank, ` + alias + `.rating_change, ` +
		alias + `.created_at`
}

func scanHistory(row interface{ Scan(...any) error }, h *model.RatingHistory) error {
	return row.Scan(&h.ID, &h.UserID, &h.ContestID, &h.RunID, &h.SupersededBy, &h.Seq,
		&h.OldRating, &h.NewRating, &h.Rank, &h.RatingChange, &h.CreatedAt)
}

// CreateHistory writes one row outside a run. Tests seed the ledger with it; a
// payout goes through ApplyRun so its rows share a run and a transaction.
func (s *RatingStore) CreateHistory(ctx context.Context, h *model.RatingHistory) error {
	if h.RunID == "" {
		return errMissingRun
	}
	return scanHistory(s.db.QueryRowContext(ctx,
		`INSERT INTO rating_history (user_id, contest_id, run_id, old_rating, new_rating, rank, rating_change)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+liveColumns,
		h.UserID, h.ContestID, h.RunID, h.OldRating, h.NewRating, h.Rank, h.RatingChange,
	), h)
}

// errMissingRun refuses a row that belongs to no payout. Every row in the ledger
// has to name the run that wrote it, or "undo this run" cannot find it.
var errMissingRun = errors.New("rating history row requires a run")

// GetByUser returns a user's live history, newest round first.
//
// Ordered by the round's place in the calendar, not by created_at: created_at is
// when the payout ran, so rating round 2 before round 1 used to put round 1 at the
// top of the graph with a base that already contained round 2.
func (s *RatingStore) GetByUser(ctx context.Context, userID string, limit int) ([]model.RatingHistory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+liveColumnsFrom("rh")+`
		   FROM rating_history rh
		   JOIN contests c ON c.id = rh.contest_id
		  WHERE rh.user_id = $1 AND rh.superseded_by IS NULL
		  ORDER BY c.start_time DESC, COALESCE(c.display_id, 0) DESC
		  LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	histories := []model.RatingHistory{}
	for rows.Next() {
		var h model.RatingHistory
		if err := scanHistory(rows, &h); err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	return histories, rows.Err()
}

// GetByContest returns the rows in force for one round, by rank.
func (s *RatingStore) GetByContest(ctx context.Context, contestID string) ([]model.RatingHistory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+liveColumns+`
		   FROM rating_history
		  WHERE contest_id = $1 AND superseded_by IS NULL
		  ORDER BY rank, seq`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	histories := []model.RatingHistory{}
	for rows.Next() {
		var h model.RatingHistory
		if err := scanHistory(rows, &h); err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	return histories, rows.Err()
}

// GetLatestByUser returns the row in force for the user's most recent round.
func (s *RatingStore) GetLatestByUser(ctx context.Context, userID string) (*model.RatingHistory, error) {
	var h model.RatingHistory
	err := scanHistory(s.db.QueryRowContext(ctx,
		`SELECT `+liveColumnsFrom("rh")+`
		   FROM rating_history rh
		   JOIN contests c ON c.id = rh.contest_id
		  WHERE rh.user_id = $1 AND rh.superseded_by IS NULL
		  ORDER BY c.start_time DESC, COALESCE(c.display_id, 0) DESC
		  LIMIT 1`, userID), &h)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// BaseRatings is each user's rating as of just before the given round, read from
// the rows still in force for earlier rounds.
//
// This is what makes a suffix replay possible at all. The ratings the replay
// starts from are the ledger's own, not a snapshot table: the rows for rounds
// before the replayed range are untouched by the replay, so they are exactly the
// state those users entered it with.
func (s *RatingStore) BaseRatings(ctx context.Context, contestID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT ON (rh.user_id) rh.user_id, rh.new_rating
		   FROM rating_history rh
		   JOIN contests c ON c.id = rh.contest_id
		  WHERE rh.superseded_by IS NULL
		    AND (c.start_time, COALESCE(c.display_id, 0)) <
		        (SELECT start_time, COALESCE(display_id, 0) FROM contests WHERE id = $1)
		  ORDER BY rh.user_id, c.start_time DESC, COALESCE(c.display_id, 0) DESC`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var userID string
		var rating int
		if err := rows.Scan(&userID, &rating); err != nil {
			return nil, err
		}
		out[userID] = rating
	}
	return out, rows.Err()
}

// CreateRun records a payout's identity. ApplyRun does this itself; this exists
// for callers that only want the run row, which is nothing yet.
func (s *RatingStore) CreateRun(ctx context.Context, run *model.RatingRun) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO rating_runs (id, contest_id, actor_id, actor_name, kind, reason)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
		runID(run), run.ContestID, nullableUUID(run.ActorID), run.ActorName, run.Kind, run.Reason,
	).Scan(&run.CreatedAt)
}

// GetRunsByContest lists the payouts that touched a round, newest first, so an
// operator can see that a round was rated, withdrawn and rated again.
func (s *RatingStore) GetRunsByContest(ctx context.Context, contestID string) ([]model.RatingRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, contest_id, COALESCE(actor_id::text, ''), actor_name, kind, reason, created_at
		   FROM rating_runs WHERE contest_id = $1 ORDER BY created_at DESC, id`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []model.RatingRun{}
	for rows.Next() {
		var r model.RatingRun
		if err := scanRun(rows, &r); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// ApplyRun writes one payout atomically: it supersedes every row in force for the
// rounds in scope, inserts the replacements, and rebuilds the touched profiles
// from what is left in force.
//
// One transaction because a half-applied payout is the worst state available: a
// superseded row with no replacement means a user's rating silently vanished, and
// a rebuilt profile that disagrees with the ledger is the drift this design
// exists to remove.
func (s *RatingStore) ApplyRun(ctx context.Context, run *model.RatingRun, scope []string, rows []model.RatingHistory) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO rating_runs (id, contest_id, actor_id, actor_name, kind, reason)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
		runID(run), run.ContestID, nullableUUID(run.ActorID), run.ActorName, run.Kind, run.Reason,
	).Scan(&run.CreatedAt); err != nil {
		return err
	}

	touched := map[string]struct{}{}

	if len(scope) > 0 {
		superseded, err := tx.QueryContext(ctx,
			`UPDATE rating_history SET superseded_by = $1
			  WHERE superseded_by IS NULL AND contest_id = ANY($2)
			  RETURNING user_id`, run.ID, pq.Array(scope))
		if err != nil {
			return err
		}
		for superseded.Next() {
			var userID string
			if err := superseded.Scan(&userID); err != nil {
				superseded.Close()
				return err
			}
			touched[userID] = struct{}{}
		}
		superseded.Close()
		if err := superseded.Err(); err != nil {
			return err
		}
	}

	for i := range rows {
		rows[i].RunID = run.ID
		if err := scanHistory(tx.QueryRowContext(ctx,
			`INSERT INTO rating_history (user_id, contest_id, run_id, old_rating, new_rating, rank, rating_change)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+liveColumns,
			rows[i].UserID, rows[i].ContestID, run.ID, rows[i].OldRating, rows[i].NewRating,
			rows[i].Rank, rows[i].RatingChange,
		), &rows[i]); err != nil {
			return err
		}
		touched[rows[i].UserID] = struct{}{}
	}

	if len(touched) == 0 {
		return tx.Commit()
	}

	userIDs := make([]string, 0, len(touched))
	for userID := range touched {
		userIDs = append(userIDs, userID)
	}

	// The profile is a cache of the ledger, rebuilt rather than incremented.
	// Incrementing is what let max_rating stay inflated after a round was
	// withdrawn: a peak cannot be decremented, only recomputed.
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_profiles up
		    SET rating = COALESCE(latest.new_rating, 0),
		        max_rating = COALESCE(agg.max_rating, 0),
		        contest_count = COALESCE(agg.rounds, 0)
		   FROM (SELECT unnest($1::uuid[]) AS user_id) t
		   LEFT JOIN LATERAL (
		        SELECT rh.new_rating
		          FROM rating_history rh
		          JOIN contests c ON c.id = rh.contest_id
		         WHERE rh.user_id = t.user_id AND rh.superseded_by IS NULL
		         ORDER BY c.start_time DESC, COALESCE(c.display_id, 0) DESC
		         LIMIT 1
		   ) latest ON TRUE
		   LEFT JOIN LATERAL (
		        SELECT MAX(rh.new_rating) AS max_rating, COUNT(*) AS rounds
		          FROM rating_history rh
		         WHERE rh.user_id = t.user_id AND rh.superseded_by IS NULL
		   ) agg ON TRUE
		  WHERE up.user_id = t.user_id`, pq.Array(userIDs)); err != nil {
		return err
	}

	return tx.Commit()
}

// ListExclusions returns the entrants disqualified from a round.
func (s *RatingStore) ListExclusions(ctx context.Context, contestID string) ([]model.RatingExclusion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, contest_id, reason, COALESCE(created_by::text, ''), created_at
		   FROM rating_exclusions WHERE contest_id = $1 ORDER BY created_at`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.RatingExclusion{}
	for rows.Next() {
		var e model.RatingExclusion
		if err := rows.Scan(&e.UserID, &e.ContestID, &e.Reason, &e.CreatedBy, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AddExclusion disqualifies one entrant from one round. Idempotent: a second call
// updates the reason rather than failing on the primary key.
func (s *RatingStore) AddExclusion(ctx context.Context, e *model.RatingExclusion) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO rating_exclusions (user_id, contest_id, reason, created_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, contest_id) DO UPDATE SET reason = EXCLUDED.reason`,
		e.UserID, e.ContestID, e.Reason, nullableUUID(e.CreatedBy))
	return err
}

// RemoveExclusion reinstates an entrant. A delete is idempotent, so an unknown
// pair is already the desired end state.
func (s *RatingStore) RemoveExclusion(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM rating_exclusions WHERE contest_id = $1 AND user_id = $2`, contestID, userID)
	return err
}

// SetContestVoided flips whether a round's ratings count.
func (s *RatingStore) SetContestVoided(ctx context.Context, contestID string, voided bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE contests SET rating_voided = $2 WHERE id = $1`, contestID, voided)
	return err
}

// RatedContestsFrom lists the rated, published rounds that have been rated from
// the given one onward, in the order a replay must walk them: the calendar.
//
// Round chronology, not run order, because the rating algorithm is a fold over
// the sequence — each round's base is the previous round's result. Ordering by
// start_time and then display_id is total: two rounds may share a start time, and
// a fold needs a tie-break that cannot change between two runs of the same replay.
func (s *ContestStore) RatedContestsFrom(ctx context.Context, contestID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM contests
		  WHERE is_rated AND status = $2
		    AND (start_time, COALESCE(display_id, 0)) >=
		        (SELECT start_time, COALESCE(display_id, 0) FROM contests WHERE id = $1)
		    -- Only rounds with ledger history: a rated round that was never paid
		    -- out is not part of the suffix, and including it would rate it as a
		    -- side effect of recomputing a different round.
		    AND EXISTS (SELECT 1 FROM rating_history rh WHERE rh.contest_id = contests.id)
		  ORDER BY start_time, COALESCE(display_id, 0)`, contestID, model.ContestStatusPublished)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// HasRatingHistory reports whether a round has any ledger rows, in force or
// superseded.
//
// The delete path needs the wider question, not "is a payout live": a round whose
// ratings were voided still owns superseded rows, and deleting it would either
// take them with it or be refused by the reference. Either way the operator meant
// something else, and 409 is the honest answer.
func (s *ContestStore) HasRatingHistory(ctx context.Context, contestID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM rating_history WHERE contest_id = $1)`,
		contestID).Scan(&exists)
	return exists, err
}

func runID(r *model.RatingRun) string {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return r.ID
}

// nullableUUID turns an optional id into a value a uuid column accepts, or nil.
// A malformed id becomes NULL rather than a 500: the column is an attribution,
// and an unattributable run is still a run.
func nullableUUID(id string) any {
	if id == "" {
		return nil
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil
	}
	return parsed
}

func scanRun(row interface{ Scan(...any) error }, r *model.RatingRun) error {
	return row.Scan(&r.ID, &r.ContestID, &r.ActorID, &r.ActorName, &r.Kind, &r.Reason, &r.CreatedAt)
}
