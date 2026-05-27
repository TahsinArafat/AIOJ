package store

import (
	"context"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type UserStore interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	ListUsers(ctx context.Context, offset, limit int) ([]model.User, int, error)
	UpdateRole(ctx context.Context, id, role string) error
}

type ProblemStore interface {
	Create(ctx context.Context, p *model.Problem) error
	GetByID(ctx context.Context, id string) (*model.Problem, error)
	GetBySlug(ctx context.Context, slug string) (*model.Problem, error)
	List(ctx context.Context, offset, limit int) ([]model.ProblemListItem, int, error)
	UpdateCounts(ctx context.Context, id string, addSubmission, addAccepted int) error
	Update(ctx context.Context, id string, p *model.Problem) error
	Delete(ctx context.Context, id string) error
	AddPermission(ctx context.Context, problemID, userID, accessLevel string) error
	RemovePermission(ctx context.Context, problemID, userID string) error
	GetPermissions(ctx context.Context, problemID string) ([]model.ProblemPermission, error)
	HasAccess(ctx context.Context, problemID, userID string, requiredLevels ...string) bool
}

type SubmissionStore interface {
	Create(ctx context.Context, s *model.Submission) error
	GetByID(ctx context.Context, id string) (*model.Submission, error)
	ListByProblem(ctx context.Context, problemID string, offset, limit int) ([]model.Submission, int, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.Submission, int, error)
	UpdateStatus(ctx context.Context, id string, status model.SubmissionStatus)
	UpdateResult(ctx context.Context, id string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) error
	ListPending(ctx context.Context, limit int) ([]string, error)
}

type RefreshTokenStore interface {
	Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	Validate(ctx context.Context, tokenHash string) (string, error)
}

type ContestStore interface {
	Create(ctx context.Context, c *model.Contest) error
	GetByID(ctx context.Context, id string) (*model.Contest, error)
	List(ctx context.Context, offset, limit int) ([]model.Contest, int, error)
	AddProblem(ctx context.Context, contestID, problemID, index string, score, sortOrder int) error
	GetProblems(ctx context.Context, contestID string) ([]model.ContestProblem, error)
	GetParticipants(ctx context.Context, contestID string) ([]string, error)
	GetUsername(ctx context.Context, userID string) string
}

type SetterStore interface {
	CreateApplication(ctx context.Context, userID, reason string) error
	ListApplications(ctx context.Context) ([]model.SetterApplication, error)
	UpdateApplicationStatus(ctx context.Context, userID, status string) error
	GetApplication(ctx context.Context, userID string) (*model.SetterApplication, error)
}
