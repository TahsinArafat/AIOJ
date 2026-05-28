package model

import "time"

const (
	GymCategoryGeneral     = "general"
	GymCategoryICPC        = "icpc"
	GymCategoryIOI         = "ioi"
	GymCategoryEducational = "educational"
	GymCategoryRegional    = "regional"
	GymCategoryNational    = "national"
	GymCategoryOpen        = "open"
)

type GymContest struct {
	ID               string    `json:"id"`
	ContestID        string    `json:"contest_id"`
	ContestTitle     string    `json:"contest_title,omitempty"`
	DifficultyRating *int      `json:"difficulty_rating,omitempty"`
	Category         string    `json:"category"`
	Country          string    `json:"country,omitempty"`
	Season           string    `json:"season,omitempty"`
	Description      string    `json:"description"`
	IsPublic         bool      `json:"is_public"`
	SolveCount       int       `json:"solve_count"`
	CreatedBy        string    `json:"created_by"`
	CreatorName      string    `json:"creator_name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreateGymRequest struct {
	ContestID        string `json:"contest_id"`
	DifficultyRating *int   `json:"difficulty_rating,omitempty"`
	Category         string `json:"category"`
	Country          string `json:"country,omitempty"`
	Season           string `json:"season,omitempty"`
	Description      string `json:"description"`
	IsPublic         bool   `json:"is_public"`
}

type GymFilter struct {
	Category  string
	MinRating *int
	MaxRating *int
	Country   string
	Search    string
}
