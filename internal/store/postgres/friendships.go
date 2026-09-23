package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tahsinarafat/aioj/internal/store"
)

type FriendshipStore struct {
	db *sql.DB
}

func NewFriendshipStore(db *sql.DB) *FriendshipStore {
	return &FriendshipStore{db: db}
}

func (s *FriendshipStore) Follow(ctx context.Context, followerID, followeeID string) (bool, error) {
	if followerID == followeeID {
		return false, fmt.Errorf("cannot follow self")
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO friendships (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, followerID, followeeID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *FriendshipStore) Unfollow(ctx context.Context, followerID, followeeID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM friendships WHERE follower_id = $1 AND followee_id = $2`,
		followerID, followeeID)
	return err
}

func (s *FriendshipStore) listEdge(ctx context.Context, col string, userID string) ([]store.FriendshipEdge, error) {
	// col is either follower_id or followee_id — fixed constants only.
	if col != "follower_id" && col != "followee_id" {
		return nil, fmt.Errorf("invalid column")
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT u.id, u.username, to_char(f.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM friendships f
		JOIN users u ON u.id = f.%s
		WHERE f.%s = $1
		ORDER BY f.created_at DESC`, otherCol(col), col), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.FriendshipEdge{}
	for rows.Next() {
		var e store.FriendshipEdge
		if err := rows.Scan(&e.UserID, &e.Username, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func otherCol(col string) string {
	if col == "follower_id" {
		return "followee_id"
	}
	return "follower_id"
}

func (s *FriendshipStore) ListFollowing(ctx context.Context, userID string) ([]store.FriendshipEdge, error) {
	return s.listEdge(ctx, "follower_id", userID)
}

func (s *FriendshipStore) ListFollowers(ctx context.Context, userID string) ([]store.FriendshipEdge, error) {
	return s.listEdge(ctx, "followee_id", userID)
}

func (s *FriendshipStore) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM friendships WHERE follower_id = $1 AND followee_id = $2`,
		followerID, followeeID).Scan(&n)
	return n > 0, err
}

func (s *FriendshipStore) Counts(ctx context.Context, userID string) (int, int, error) {
	var following, followers int
	err := s.db.QueryRowContext(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE follower_id = $1),
			COUNT(*) FILTER (WHERE followee_id = $1)
		 FROM friendships WHERE follower_id = $1 OR followee_id = $1`,
		userID).Scan(&following, &followers)
	return following, followers, err
}

var _ store.FriendshipStore = (*FriendshipStore)(nil)

// ensure time import used (to_char returns string; CreatedAt is string in edge)
var _ = time.Time{}
