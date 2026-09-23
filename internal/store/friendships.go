package store

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/model"
)

// AchievementStore awards and lists user badges.
type AchievementStore interface {
	ListForUser(ctx context.Context, userID string) ([]model.UserAchievement, error)
	Award(ctx context.Context, userID, code string) (bool, error)
	AwardProgress(ctx context.Context, userID string, solved int) error
}

// FriendshipStore manages follow edges.
type FriendshipStore interface {
	Follow(ctx context.Context, followerID, followeeID string) (bool, error)
	Unfollow(ctx context.Context, followerID, followeeID string) error
	ListFollowing(ctx context.Context, userID string) ([]FriendshipEdge, error)
	ListFollowers(ctx context.Context, userID string) ([]FriendshipEdge, error)
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
	Counts(ctx context.Context, userID string) (following, followers int, err error)
}

// FriendshipEdge is a follow relationship with the other user's username.
type FriendshipEdge struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}

// AuditStore records admin audit entries (Phase E).
type AuditStore interface {
	Insert(ctx context.Context, e *model.AuditEntry) error
	List(ctx context.Context, offset, limit int) ([]model.AuditEntry, int, error)
}
