package api

import (
	"github.com/tahsinarafat/aioj/internal/api/handler"
)

// Deps holds all HTTP handler dependencies for the router.
// Add a new field here when registering a new handler — no function signature change needed.
type Deps struct {
	Auth           *handler.AuthHandler
	Problem        *handler.ProblemHandler
	Submission     *handler.SubmissionHandler
	Contest        *handler.ContestHandler
	ContestProblem *handler.ContestProblemHandler
	VJudge         *handler.VJudgeHandler
	Admin          *handler.AdminHandler
	Testcase       *handler.TestcaseHandler
	WS             *handler.WSManager
	Rating         *handler.RatingHandler
	Registration   *handler.RegistrationHandler
	Virtual        *handler.VirtualHandler
	Gym            *handler.GymHandler
	Hack           *handler.HackHandler
	Stats          *handler.StatsHandler
	Notification   *handler.NotificationHandler
	Group          *handler.GroupHandler
	Team           *handler.TeamHandler
	Blog           *handler.BlogHandler
	Editorial      *handler.EditorialHandler
	APIKey         *handler.APIKeyHandler
	Webhook        *handler.WebhookHandler
	Recommendation *handler.RecommendationHandler
	Rankings       *handler.RankingsHandler
	Users          *handler.UsersHandler
	Search         *handler.SearchHandler
	LangLimit      *handler.LanguageLimitHandler
	Import         *handler.ImportHandler
	Org            *handler.OrganizationHandler
	Class          *handler.ClassHandler
	Training       *handler.TrainingHandler
	Plagiarism     *handler.PlagiarismHandler
	Media          *handler.MediaHandler
	Onsite         *handler.OnsiteHandler
	OnsiteBatch    *handler.OnsiteBatchHandler
	Clarification  *handler.ClarificationHandler
	Notice         *handler.ContestNoticeHandler
	BotAccount     *handler.AdminBotAccountHandler
	Settings       *handler.AdminSystemSettingsHandler
	LangAdmin      *handler.AdminLanguageHandler
	RemoteLang     *handler.RemoteLanguageHandler
	AdminSub       *handler.AdminSubmissionHandler
	Backup         *handler.AdminBackupHandler
}
