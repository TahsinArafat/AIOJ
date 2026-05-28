package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type NotificationStore struct {
	db *sql.DB
}

func NewNotificationStore(db *sql.DB) *NotificationStore {
	return &NotificationStore{db: db}
}

func (s *NotificationStore) Create(ctx context.Context, n *model.Notification) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO notifications (user_id, type, title, content, link)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		n.UserID, n.Type, n.Title, n.Content, n.Link,
	).Scan(&n.ID, &n.CreatedAt)
}

func (s *NotificationStore) GetByUser(ctx context.Context, userID string, unreadOnly bool, limit int) ([]model.Notification, error) {
	query := `SELECT id, user_id, type, title, content, link, read, created_at
	          FROM notifications WHERE user_id = $1`
	args := []interface{}{userID}

	if unreadOnly {
		query += " AND read = false"
	}

	query += " ORDER BY created_at DESC LIMIT $2"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &n.Link, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []model.Notification{}
	}
	return notifications, nil
}

func (s *NotificationStore) MarkAsRead(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE notifications SET read = true WHERE id = $1", id)
	return err
}

func (s *NotificationStore) MarkAllAsRead(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE notifications SET read = true WHERE user_id = $1 AND read = false", userID)
	return err
}

func (s *NotificationStore) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false",
		userID).Scan(&count)
	return count, err
}

func (s *NotificationStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM notifications WHERE id = $1", id)
	return err
}

func (s *NotificationStore) GetPreferences(ctx context.Context, userID string) (*model.NotificationPreferences, error) {
	var p model.NotificationPreferences
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, contest_announcements, rating_changes, hack_notifications, 
		        group_activities, email_digest
		 FROM notification_preferences WHERE user_id = $1`,
		userID).Scan(&p.UserID, &p.ContestAnnouncements, &p.RatingChanges,
		&p.HackNotifications, &p.GroupActivities, &p.EmailDigest)
	if err == sql.ErrNoRows {
		return &model.NotificationPreferences{
			UserID:               userID,
			ContestAnnouncements: true,
			RatingChanges:        true,
			HackNotifications:    true,
			GroupActivities:      true,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *NotificationStore) UpdatePreferences(ctx context.Context, userID string, prefs *model.NotificationPreferences) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notification_preferences (user_id, contest_announcements, rating_changes, 
		         hack_notifications, group_activities, email_digest)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id) DO UPDATE SET
		         contest_announcements = $2, rating_changes = $3,
		         hack_notifications = $4, group_activities = $5, email_digest = $6`,
		userID, prefs.ContestAnnouncements, prefs.RatingChanges,
		prefs.HackNotifications, prefs.GroupActivities, prefs.EmailDigest)
	return err
}
