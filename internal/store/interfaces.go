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
	ListWithFilter(ctx context.Context, offset, limit int, difficulty string, tags []string, search string) ([]model.ProblemListItem, int, error)
	GetAllTags(ctx context.Context) ([]string, error)
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
	GetProblemStats(ctx context.Context, problemID string) (*model.ProblemStats, error)
	GetUserStats(ctx context.Context, userID string) (*model.UserProblemStats, error)
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
	AddPermission(ctx context.Context, contestID, userID, accessLevel string) error
	RemovePermission(ctx context.Context, contestID, userID string) error
	GetPermissions(ctx context.Context, contestID string) ([]model.ContestPermission, error)
	HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool
}

type RatingStore interface {
	CreateHistory(ctx context.Context, h *model.RatingHistory) error
	GetByUser(ctx context.Context, userID string, limit int) ([]model.RatingHistory, error)
	GetByContest(ctx context.Context, contestID string) ([]model.RatingHistory, error)
	GetLatestByUser(ctx context.Context, userID string) (*model.RatingHistory, error)
}

type RegistrationStore interface {
	Register(ctx context.Context, contestID, userID string) error
	Unregister(ctx context.Context, contestID, userID string) error
	IsRegistered(ctx context.Context, contestID, userID string) (bool, error)
	GetRegistrations(ctx context.Context, contestID string) ([]model.ContestRegistration, error)
	GetRegistrationCount(ctx context.Context, contestID string) (int, error)
}

type SetterStore interface {
	CreateApplication(ctx context.Context, userID, reason string) error
	ListApplications(ctx context.Context) ([]model.SetterApplication, error)
	UpdateApplicationStatus(ctx context.Context, userID, status string) error
	GetApplication(ctx context.Context, userID string) (*model.SetterApplication, error)
}

type VirtualStore interface {
	Create(ctx context.Context, v *model.VirtualContest) error
	GetByID(ctx context.Context, id string) (*model.VirtualContest, error)
	GetActiveByUser(ctx context.Context, userID string) (*model.VirtualContest, error)
	Complete(ctx context.Context, id string) error
}

type GymStore interface {
	Create(ctx context.Context, g *model.GymContest) error
	GetByID(ctx context.Context, id string) (*model.GymContest, error)
	List(ctx context.Context, offset, limit int, filter model.GymFilter) ([]model.GymContest, int, error)
	MarkSolved(ctx context.Context, gymID, userID string) error
	IsSolved(ctx context.Context, gymID, userID string) (bool, error)
}

type HackStore interface {
	Create(ctx context.Context, h *model.Hack) error
	GetByID(ctx context.Context, id string) (*model.Hack, error)
	UpdateStatus(ctx context.Context, id, status string, success bool) error
	GetByContest(ctx context.Context, contestID string) ([]model.Hack, error)
	GetHackableSubmissions(ctx context.Context, contestID, problemID string) ([]model.Submission, error)
}

type NotificationStore interface {
	Create(ctx context.Context, n *model.Notification) error
	GetByUser(ctx context.Context, userID string, unreadOnly bool, limit int) ([]model.Notification, error)
	MarkAsRead(ctx context.Context, id string) error
	MarkAllAsRead(ctx context.Context, userID string) error
	GetUnreadCount(ctx context.Context, userID string) (int, error)
	Delete(ctx context.Context, id string) error
	GetPreferences(ctx context.Context, userID string) (*model.NotificationPreferences, error)
	UpdatePreferences(ctx context.Context, userID string, prefs *model.NotificationPreferences) error
}

type GroupStore interface {
	Create(ctx context.Context, g *model.Group) error
	GetByID(ctx context.Context, id string) (*model.Group, error)
	List(ctx context.Context, offset, limit int) ([]model.GroupListItem, int, error)
	ListByUser(ctx context.Context, userID string) ([]model.GroupListItem, error)
	Update(ctx context.Context, id string, g *model.Group) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, groupID, userID, role string) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	GetMembers(ctx context.Context, groupID string) ([]model.GroupMember, error)
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	GetMemberCount(ctx context.Context, groupID string) (int, error)
	AddContest(ctx context.Context, groupID, contestID string) error
	RemoveContest(ctx context.Context, groupID, contestID string) error
	GetContests(ctx context.Context, groupID string) ([]model.Contest, error)
}

type TeamStore interface {
	Create(ctx context.Context, t *model.Team) error
	GetByID(ctx context.Context, id string) (*model.Team, error)
	List(ctx context.Context, offset, limit int) ([]model.TeamListItem, int, error)
	Update(ctx context.Context, id string, t *model.Team) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, teamID, userID, role string) error
	RemoveMember(ctx context.Context, teamID, userID string) error
	GetMembers(ctx context.Context, teamID string) ([]model.TeamMember, error)
	IsMember(ctx context.Context, teamID, userID string) (bool, error)
	GetUserTeams(ctx context.Context, userID string) ([]model.TeamListItem, error)
	UpdateRating(ctx context.Context, teamID string, newRating int) error
}
