package model

import "time"

type TrainingPlan struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	OrganizationID *string   `json:"organization_id,omitempty"`
	OrgName        string    `json:"org_name,omitempty"`
	IsPublic       bool      `json:"is_public"`
	CreatedBy      string    `json:"created_by"`
	CreatorName    string    `json:"creator_name,omitempty"`
	SectionCount   int       `json:"section_count"`
	ProblemCount   int       `json:"problem_count"`
	EnrolledCount  int       `json:"enrolled_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TrainingPlanSection struct {
	ID          string                `json:"id"`
	PlanID      string                `json:"plan_id"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	SortOrder   int                   `json:"sort_order"`
	Problems    []TrainingPlanProblem `json:"problems,omitempty"`
}

type TrainingPlanProblem struct {
	ID        string `json:"id"`
	SectionID string `json:"section_id"`
	ProblemID string `json:"problem_id"`
	SortOrder int    `json:"sort_order"`
	Points    int    `json:"points"`
}

type TrainingPlanEnrollment struct {
	PlanID     string    `json:"plan_id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username,omitempty"`
	EnrolledAt time.Time `json:"enrolled_at"`
}

type TrainingPlanProgress struct {
	PlanID      string     `json:"plan_id"`
	UserID      string     `json:"user_id"`
	ProblemID   string     `json:"problem_id"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type CreateTrainingPlanRequest struct {
	Title          string                   `json:"title"`
	Description    string                   `json:"description"`
	OrganizationID *string                  `json:"organization_id,omitempty"`
	Sections       []CreateSectionRequest   `json:"sections"`
}

type CreateSectionRequest struct {
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Problems    []CreateProblemEntryRequest `json:"problems"`
}

type CreateProblemEntryRequest struct {
	ProblemID string `json:"problem_id"`
	Points    int    `json:"points"`
}

type TrainingPlanDetail struct {
	TrainingPlan
	Sections  []TrainingPlanSection `json:"sections"`
	Enrolled  bool                  `json:"enrolled"`
	Progress  *PlanProgressSummary  `json:"progress,omitempty"`
}

type PlanProgressSummary struct {
	TotalProblems     int     `json:"total_problems"`
	CompletedProblems int     `json:"completed_problems"`
	Percentage        float64 `json:"percentage"`
}
