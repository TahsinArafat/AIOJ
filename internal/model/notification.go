package model

import "time"

const (
	NotificationTypeContest = "contest"
	NotificationTypeRating  = "rating"
	NotificationTypeHack    = "hack"
	NotificationTypeGroup   = "group"
	NotificationTypeSystem  = "system"
)

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Link      string    `json:"link,omitempty"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type NotificationPreferences struct {
	UserID               string `json:"user_id"`
	ContestAnnouncements bool   `json:"contest_announcements"`
	RatingChanges        bool   `json:"rating_changes"`
	HackNotifications    bool   `json:"hack_notifications"`
	GroupActivities      bool   `json:"group_activities"`
	EmailDigest          bool   `json:"email_digest"`
}

type CreateNotificationRequest struct {
	UserID  string `json:"user_id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Link    string `json:"link,omitempty"`
}
