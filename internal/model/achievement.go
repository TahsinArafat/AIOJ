package model

import "time"

// Achievement is a catalog badge definition.
type Achievement struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserAchievement is a awarded badge for a user.
type UserAchievement struct {
	AchievementID string    `json:"achievement_id"`
	Code          string    `json:"code"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Icon          string    `json:"icon"`
	AwardedAt     time.Time `json:"awarded_at"`
}
