package api

import (
	"context"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/feature"
	"github.com/tahsinarafat/aioj/internal/model"
)

// Deps holds all HTTP handler dependencies for the router.
// Add a new field here when registering a new handler — no function signature change needed.
type Deps struct {
	Auth        *handler.AuthHandler
	Problem     *handler.ProblemHandler
	ProblemI18n *handler.ProblemI18nHandler
	Submission  *handler.SubmissionHandler
	Contest     *handler.ContestHandler
	// ContestUnlock redeems a round's password for an access grant.
	ContestUnlock  *handler.ContestUnlockHandler
	ContestProblem *handler.ContestProblemHandler
	// ContestAdjustment is the jury's manual-correction ledger for standings.
	ContestAdjustment *handler.ContestAdjustmentHandler
	VJudge            *handler.VJudgeHandler
	Admin             *handler.AdminHandler
	Testcase          *handler.TestcaseHandler
	WS                *handler.WSManager
	Rating            *handler.RatingHandler
	Registration      *handler.RegistrationHandler
	Virtual           *handler.VirtualHandler
	Gym               *handler.GymHandler
	Hack              *handler.HackHandler
	Stats             *handler.StatsHandler
	Notification      *handler.NotificationHandler
	Group             *handler.GroupHandler
	Team              *handler.TeamHandler
	Blog              *handler.BlogHandler
	Editorial         *handler.EditorialHandler
	APIKey            *handler.APIKeyHandler
	Webhook           *handler.WebhookHandler
	Recommendation    *handler.RecommendationHandler
	Rankings          *handler.RankingsHandler
	Users             *handler.UsersHandler
	Search            *handler.SearchHandler
	LangLimit         *handler.LanguageLimitHandler
	Import            *handler.ImportHandler
	Org               *handler.OrganizationHandler
	Class             *handler.ClassHandler
	Training          *handler.TrainingHandler
	Plagiarism        *handler.PlagiarismHandler
	Onsite            *handler.OnsiteHandler
	OnsiteBatch       *handler.OnsiteBatchHandler
	Clarification     *handler.ClarificationHandler
	Notice            *handler.ContestNoticeHandler
	BotAccount        *handler.AdminBotAccountHandler
	Settings          *handler.AdminSystemSettingsHandler
	LangAdmin         *handler.AdminLanguageHandler
	RemoteLang        *handler.RemoteLanguageHandler
	AdminSub          *handler.AdminSubmissionHandler
	Backup            *handler.AdminBackupHandler
	AIModel           *handler.AdminAIModelHandler
	// Features serves the feature-gate registry and its admin CRUD.
	Features *handler.FeaturesHandler
	// FeatureResolver is the same resolver the handler writes through; the
	// router mounts it as per-route gates.
	FeatureResolver *feature.Resolver
	CSRFSecret      string
	AIEnabled       bool
	// MetricsToken guards /metrics. Empty means only an admin access token can
	// scrape it (anonymous scraping is never allowed).
	MetricsToken string
	// Optional holds the dependencies a deployment may legitimately leave unset.
	// Each is guarded by an `if b.Optional.X != nil` in its mount file, so an
	// absent one means a route is not registered rather than a nil handler being
	// reachable.
	//
	// Grouped into its own type rather than left inline so the distinction is in
	// the type rather than in a list maintained beside it. ValidateWiring checks
	// only the fields above; a field here is optional by construction.
	Optional OptionalDeps
}

// OptionalDeps holds the dependencies a deployment may leave unset.
type OptionalDeps struct {
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
	CommentFeed func(ctx context.Context) ([]handler.FeedItem, error)
	// Meta renders the crawler-facing copy of an app route with per-page
	// Open Graph tags. nginx routes social-network user agents to it, because
	// those crawlers do not execute JavaScript and so never see the SPA's
	// client-set page title.
	Meta *handler.MetaHandler
	// APIKeys and Users back the X-API-Key middleware. They are the stores
	// themselves rather than handlers because the middleware runs before any
	// handler and needs no HTTP surface of its own.
	APIKeys     APIVerifier
	APIKeyUsers APIUserLookup
	Media       *handler.MediaHandler
	// PublicLanguages serves the judge's language list to the SPA, so the IDE
	// and the submit forms offer exactly what lang/*.yaml defines.
	PublicLanguages *handler.PublicLanguageHandler
	CDN             *handler.CDNHandler
	Achievements    *handler.AchievementHandler
	Friendships     *handler.FriendshipHandler
	AuditLog        *handler.AuditLogHandler
	// Health is optional; when set, /api/health reports dependency status.
	Health        *handler.HealthChecker
	Activity      *handler.ActivityHandler
	AIGeneration  *handler.AIGenerationHandler
	DevMail       *handler.DevMailHandler
	VerifyEmail   *handler.EmailVerificationHandler
	TwoFA         *handler.TwoFactorHandler
	TwoFAVerify   *handler.TwoFactorVerifyHandler
	OAuthStart    *handler.OAuthStartHandler
	OAuthCallback *handler.OAuthCallbackHandler
	// AuthMethods reports which sign-in methods are usable, and serves the
	// admin switches behind them.
	AuthMethods   *handler.AuthMethodsHandler
	Legal         *handler.LegalHandler
	UsersExport   *handler.UsersExportHandler
	UsersDeletion *handler.UsersDeletionHandler
}

// APIVerifier is the key store surface the API-key middleware needs. Declared
// here rather than in the middleware package so the router's Deps stays the one
// place that knows which stores exist.
type APIVerifier interface {
	GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error)
	UpdateLastUsed(ctx context.Context, id string) error
	IncrementRequestCount(ctx context.Context, keyID string, windowStart time.Time) error
	GetRequestCount(ctx context.Context, keyID string, windowStart time.Time) (int, error)
}

// APIUserLookup resolves the owner of an API key.
type APIUserLookup interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}
