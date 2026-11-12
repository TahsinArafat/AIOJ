package postgres

import (
	"context"
	"database/sql"
)

// ContestGrantStore is the persistent proof that a contestant unlocked a
// password-protected round. It is what stops "the password was never checked"
// from recurring: the entitlement is a row, so the read path, the submit path and
// the scoreboard all consult the same fact instead of each carrying a flag.
type ContestGrantStore struct{ db *sql.DB }

func NewContestGrantStore(db *sql.DB) *ContestGrantStore { return &ContestGrantStore{db: db} }

// Grant records the entitlement. ON CONFLICT DO NOTHING makes a second unlock a
// no-op rather than an error: a contestant re-entering the password, or two tabs
// racing, must not produce a failure the UI has to interpret.
func (s *ContestGrantStore) Grant(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_access_grants (contest_id, user_id)
		 VALUES ($1, $2)
		 ON CONFLICT (contest_id, user_id) DO NOTHING`,
		contestID, userID)
	return err
}

func (s *ContestGrantStore) IsGranted(ctx context.Context, contestID, userID string) (bool, error) {
	// An empty user id cannot own a grant. Answering early keeps anonymous
	// callers out of a query that would only ever return false, and avoids
	// relying on the uuid cast to reject ''.
	if userID == "" {
		return false, nil
	}
	var granted bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM contest_access_grants WHERE contest_id = $1 AND user_id = $2)`,
		contestID, userID).Scan(&granted)
	return granted, err
}

// RevokeAll drops a round's entitlements. Called when the setter changes the
// password: the grants were proof of the OLD secret, so keeping them would let a
// contestant who was told the previous password stay in the round forever.
func (s *ContestGrantStore) RevokeAll(ctx context.Context, contestID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM contest_access_grants WHERE contest_id = $1`, contestID)
	return err
}
