package api

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/api/handler"
)

// Deps holds all HTTP handler dependencies for the router.
// Add a new field here when registering a new handler — no function signature change needed.
type Deps struct {
	Auth        *handler.AuthHandler
	Problem     *handler.ProblemHandler
	ProblemI18n *handler.ProblemI18nHandler
	// Sitemap returns the URL groups for /sitemap.xml. A func rather than a
	// handler type so the DB-backed gathering stays in main.go where the stores
	// live, and the router stays free of store knowledge.
	Sitemap func(ctx context.Context) []handler.SitemapSection
	// Feed returns the items for the problems Atom feed, plus any fetch error.
	// The error is passed through rather than logged so a failed query cannot
	// be served as a valid empty feed.
	Feed func(ctx context.Context) ([]handler.FeedItem, error)
	// BlogFeed returns items for the blog Atom feed, plus any fetch error.
	BlogFeed func(ctx context.Context) ([]handler.FeedItem, error)
	// ContestFeed returns items for the contests Atom feed, plus any fetch error.
	ContestFeed func(ctx context.Context) ([]handler.FeedItem, error)
	// CommentFeed returns items for the comments Atom feed, plus any fetch error.
	CommentFeed    func(ctx context.Context) ([]handler.FeedItem, error)
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
	Generate       *handler.GenerateHandler
	AIModel        *handler.AdminAIModelHandler
	DevMail        *handler.DevMailHandler
	VerifyEmail    *handler.EmailVerificationHandler
	TwoFA          *handler.TwoFactorHandler
	TwoFAVerify    *handler.TwoFactorVerifyHandler
	OAuthStart     *handler.OAuthStartHandler
	OAuthCallback  *handler.OAuthCallbackHandler
	Legal          *handler.LegalHandler
	UsersExport    *handler.UsersExportHandler
	UsersDeletion  *handler.UsersDeletionHandler
	CSRFSecret     string
}
