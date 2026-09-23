package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/tahsinarafat/aioj/internal/api"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/config"
	"github.com/tahsinarafat/aioj/internal/generate"
	"github.com/tahsinarafat/aioj/internal/hack"
	"github.com/tahsinarafat/aioj/internal/judge"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"github.com/tahsinarafat/aioj/internal/observability"
	"github.com/tahsinarafat/aioj/internal/plagiarism"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"github.com/tahsinarafat/aioj/internal/virtual"
	"github.com/tahsinarafat/aioj/internal/vjudge"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	mode := flag.String("mode", "server", "run mode: server|judge-worker")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("starting aioj", "port", cfg.Server.Port)
	defer observability.InitSentry()()

	db, err := postgres.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	slog.Info("running database migrations...")
	m, err := migrate.New("file://internal/store/migrations", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name))
	if err != nil {
		log.Fatalf("failed to initialize migrations: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to apply migrations: %v", err)
	}
	slog.Info("database migrations applied successfully")

	accessTTL, _ := time.ParseDuration(cfg.Auth.AccessTTL)
	refreshTTL, _ := time.ParseDuration(cfg.Auth.RefreshTTL)
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, accessTTL, refreshTTL)

	userStore := postgres.NewUserStore(db)
	refreshTokenStore := postgres.NewRefreshTokenStore(db)
	problemStore := postgres.NewProblemStore(db)
	problemI18nStore := postgres.NewProblemI18nStore(db)
	sitemapStore := postgres.NewSitemapStore(db)
	feedStore := postgres.NewFeedStore(db)
	submissionStore := postgres.NewSubmissionStore(db)
	contestStore := postgres.NewContestStore(db)
	ratingStore := postgres.NewRatingStore(db)

	var judgeQueue queue.JudgeQueue = queue.NewMemory()
	if cfg.Redis.URL != "" {
		redisClient := redis.NewClient(&redis.Options{Addr: cfg.Redis.URL})
		judgeQueue = queue.NewRedisQueue(redisClient)
		slog.Info("using redis judge queue", "url", cfg.Redis.URL)
	}
	execClient := executor.NewClient(cfg.Judge.Endpoint)
	langLimitStore := postgres.NewLanguageLimitStore(db)
	langLimitH := handler.NewLanguageLimitHandler(langLimitStore, problemStore)
	balloonStore := postgres.NewBalloonStore(db)
	printStore := postgres.NewPrintStore(db)
	workerPool := judge.NewWorkerPool(judgeQueue, execClient, cfg.LangDir, cfg.Judge.Concurrency, submissionStore, problemStore, langLimitStore, balloonStore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go workerPool.Start(ctx)

	if *mode == "judge-worker" {
		slog.Info("running in judge-worker mode, waiting for submissions")
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("judge-worker shutting down")
		return
	}

	wsManager := handler.NewWSManager()
	passwordResetTokenStore := postgres.NewPasswordResetTokenStore(db)
	onsiteUserStore := postgres.NewOnsiteUserStore(db)
	evtStore := postgres.NewEmailVerificationTokenStore(db)
	totpStore := postgres.NewTOTPSecretStore(db)
	oauthLinkStore := postgres.NewOAuthLinkStore(db)
	backupCodeStore := postgres.NewBackupCodeStore(db)

	var mailSender mail.Sender
	switch cfg.Mail.Driver {
	case "smtp":
		mailSender = mail.NewSMTPSender(mail.SMTPConfig{
			Host: cfg.Mail.Host, Port: cfg.Mail.Port,
			Username: cfg.Mail.Username, Password: cfg.Mail.Password,
			From: cfg.Mail.From,
		})
	case "catcher":
		mailSender = mail.NewMailCatcher()
	default:
		mailSender = mail.NoopSender{}
	}
	mailTpl, err := mail.LoadTemplates()
	if err != nil {
		log.Fatalf("load mail templates: %v", err)
	}
	publicURL := cfg.Mail.PublicURL
	if publicURL == "" {
		publicURL = "http://localhost:8081"
	}
	mailFrom := cfg.Mail.From
	if mailFrom == "" {
		mailFrom = "noreply@aioj.com"
	}

	authH := handler.NewAuthHandler(userStore, refreshTokenStore, passwordResetTokenStore, onsiteUserStore, contestStore, jwtManager, evtStore, totpStore, mailSender, mailTpl, publicURL, mailFrom)
	problemH := handler.NewProblemHandler(problemStore)
	problemI18nH := handler.NewProblemI18nHandler(problemI18nStore, problemStore)

	botAccountStore := postgres.NewBotAccountStore(db)
	remoteLangStore := postgres.NewRemoteLanguageStore(db)
	vjService := vjudge.NewService(submissionStore, problemStore, botAccountStore, remoteLangStore)

	cfSubmitURL := os.Getenv("CF_SUBMIT_URL")
	if cfSubmitURL == "" {
		cfSubmitURL = "http://host.docker.internal:8003"
	}
	cfSubmit := vjudge.NewCFSubmitClient(cfSubmitURL)

	atcoderSubmitURL := os.Getenv("ATCODER_SUBMIT_URL")
	if atcoderSubmitURL == "" {
		atcoderSubmitURL = "http://host.docker.internal:8004"
	}
	atcoderSubmit := vjudge.NewAtCoderSubmitClient(atcoderSubmitURL)

	submissionH := handler.NewSubmissionHandler(submissionStore, problemStore, contestStore, judgeQueue, wsManager, execClient, cfg.LangDir, vjService, userStore)
	teamStore := postgres.NewTeamStore(db)
	contestH := handler.NewContestHandler(contestStore, ratingStore, problemStore, userStore, teamStore)
	contestProblemH := handler.NewContestProblemHandler(contestStore, problemStore)

	platforms := []string{"codeforces", "atcoder", "cses", "toph", "qoj"}
	for _, platform := range platforms {
		accounts, _ := botAccountStore.ListByPlatform(ctx, platform)
		vjCfg := vjudge.BotConfig{}
		if len(accounts) > 0 && accounts[0].Status == "active" {
			vjCfg.Username = accounts[0].PlatformUser
			vjCfg.Password = accounts[0].PlatformPass
			vjCfg.APIKey = accounts[0].APIKey
			vjCfg.APISecret = accounts[0].APISecret
			vjCfg.Cookies = accounts[0].SessionData
		}
		switch platform {
		case "codeforces":
			vjService.RegisterBot(platform, vjudge.NewCodeforcesBotWithSubmit(vjCfg, cfSubmit))
		case "atcoder":
			vjService.RegisterBot(platform, vjudge.NewAtCoderBotWithSubmit(vjCfg, atcoderSubmit))
		case "cses":
			vjService.RegisterBot(platform, vjudge.NewCSESBotWithStore(vjCfg, remoteLangStore))
		case "toph":
			vjService.RegisterBot(platform, vjudge.NewTophBot(vjCfg))
		case "qoj":
			vjService.RegisterBot(platform, vjudge.NewQOJBot(vjCfg))
		}
	}

	vjService.StartPollWorkers()
	vjService.StartSubmitWorkers()

	vjH := handler.NewVJudgeHandler(vjService)

	setterStore := postgres.NewSetterStore(db)
	adminH := handler.NewAdminHandler(userStore, setterStore)

	testcaseH := handler.NewTestcaseHandler(problemStore, "./testdata")
	importH := handler.NewImportHandler(problemStore, "./testdata")
	ratingH := handler.NewRatingHandler(ratingStore)
	registrationStore := postgres.NewRegistrationStore(db)
	registrationH := handler.NewRegistrationHandler(registrationStore, contestStore)
	virtualStore := postgres.NewVirtualStore(db)
	virtualService := virtual.NewService(virtualStore)
	virtualH := handler.NewVirtualHandler(virtualService, virtualStore)
	gymStore := postgres.NewGymStore(db)
	gymH := handler.NewGymHandler(gymStore)
	hackStore := postgres.NewHackStore(db)
	hackService := hack.NewService(hackStore, contestStore, submissionStore)
	hackH := handler.NewHackHandler(hackService, hackStore)
	statsH := handler.NewStatsHandler(submissionStore)
	notifStore := postgres.NewNotificationStore(db)
	notifH := handler.NewNotificationHandler(notifStore)
	groupStore := postgres.NewGroupStore(db)
	groupH := handler.NewGroupHandler(groupStore, userStore)
	teamH := handler.NewTeamHandler(teamStore, userStore)
	blogStore := postgres.NewBlogStore(db)
	blogH := handler.NewBlogHandler(blogStore)
	editorialStore := postgres.NewEditorialStore(db)
	editorialH := handler.NewEditorialHandler(editorialStore)
	apiKeyStore := postgres.NewAPIKeyStore(db)
	apiKeyH := handler.NewAPIKeyHandler(apiKeyStore)
	webhookStore := postgres.NewWebhookStore(db)
	webhookH := handler.NewWebhookHandler(webhookStore)
	recommendationH := handler.NewRecommendationHandler(problemStore, ratingStore)
	rankingsH := handler.NewRankingsHandler(userStore)
	usersH := handler.NewUsersHandler(userStore, blogStore, submissionStore, teamStore, groupStore)
	searchStore := postgres.NewSearchStore(db)
	searchH := handler.NewSearchHandler(searchStore)

	orgStore := postgres.NewOrganizationStore(db)
	classStore := postgres.NewClassStore(db)
	trainingStore := postgres.NewTrainingPlanStore(db)

	orgH := handler.NewOrganizationHandler(orgStore)
	classH := handler.NewClassHandler(classStore, orgStore)
	trainingH := handler.NewTrainingHandler(trainingStore, orgStore)

	plagiarismStore := postgres.NewPlagiarismStore(db)
	plagiarismService := plagiarism.NewService(plagiarismStore, contestStore, submissionStore)
	plagiarismH := handler.NewPlagiarismHandler(plagiarismService, plagiarismStore, contestStore)

	mediaH := handler.NewMediaHandler("./media")
	onsiteH := handler.NewOnsiteHandler(balloonStore, printStore, contestStore)
	onsiteBatchH := handler.NewOnsiteBatchHandler(contestStore, onsiteUserStore, userStore, refreshTokenStore, jwtManager)

	clarificationStore := postgres.NewClarificationStore(db)
	clarificationH := handler.NewClarificationHandler(clarificationStore, contestStore)
	noticeStore := postgres.NewContestNoticeStore(db)
	noticeH := handler.NewContestNoticeHandler(noticeStore, contestStore)

	botAccountH := handler.NewAdminBotAccountHandler(botAccountStore, vjService)
	settingsStore := postgres.NewSystemSettingsStore(db)
	settingsH := handler.NewAdminSystemSettingsHandler(settingsStore)
	langAdminH := handler.NewAdminLanguageHandler(cfg.LangDir)

	remoteLangH := handler.NewRemoteLanguageHandler(remoteLangStore, vjService)
	adminSubH := handler.NewAdminSubmissionHandler(submissionStore, problemStore, vjService)
	backupH := handler.NewAdminBackupHandler(userStore, cfg.Database, "./backups")

	aiModelStore := postgres.NewAIModelStore(db)
	aiModelH := handler.NewAdminAIModelHandler(aiModelStore)
	genSvc := generate.NewService(aiModelStore, problemStore, editorialStore)
	generateH := handler.NewGenerateHandler(genSvc)

	verifyH := &handler.EmailVerificationHandler{Users: userStore, Tokens: evtStore}
	var devMailH *handler.DevMailHandler
	if cfg.Mail.Driver == "catcher" {
		devMailH = &handler.DevMailHandler{Sender: mailSender}
	}

	twoFAH := &handler.TwoFactorHandler{Users: userStore, Secrets: totpStore, Backups: backupCodeStore}
	twoFAVerifyH := &handler.TwoFactorVerifyHandler{Secrets: totpStore, Backups: backupCodeStore, JWT: jwtManager, Users: userStore, Refresh: refreshTokenStore}

	// OAuth providers (empty client_id disables the start redirect until configured).
	publicOrigin := cfg.Mail.PublicURL
	if publicOrigin == "" {
		publicOrigin = "http://localhost:8081"
	}
	oauthProviders := map[string]oauth.Provider{
		"github": oauth.NewGitHubProvider(oauth.GitHubConfig{}),
		"google": oauth.NewGoogleProvider(oauth.GoogleConfig{}),
	}
	if cfg.OAuth.GitHub.ClientID != "" {
		gh := oauthProviders["github"].(*oauth.GitHubProvider)
		gh.Config().ClientID = cfg.OAuth.GitHub.ClientID
		gh.Config().ClientSecret = cfg.OAuth.GitHub.ClientSecret
		ru := cfg.OAuth.GitHub.RedirectURL
		if ru == "" {
			ru = strings.TrimRight(publicOrigin, "/") + "/api/auth/oauth/github/callback"
		}
		gh.Config().RedirectURL = ru
	}
	if cfg.OAuth.Google.ClientID != "" {
		gp := oauthProviders["google"].(*oauth.GoogleProvider)
		gp.Config().ClientID = cfg.OAuth.Google.ClientID
		gp.Config().ClientSecret = cfg.OAuth.Google.ClientSecret
		ru := cfg.OAuth.Google.RedirectURL
		if ru == "" {
			ru = strings.TrimRight(publicOrigin, "/") + "/api/auth/oauth/google/callback"
		}
		gp.Config().RedirectURL = ru
	}
	stateSecret := []byte(cfg.OAuth.StateSecret)
	if len(stateSecret) == 0 {
		stateSecret = []byte(cfg.Auth.CSRFSecret)
	}
	oauthStartH := &handler.OAuthStartHandler{
		Providers: oauthProviders, StateSecret: stateSecret, StateTTL: 10 * time.Minute,
	}
	oauthCallbackH := &handler.OAuthCallbackHandler{
		Users: userStore, Links: oauthLinkStore, Providers: oauthProviders,
		JWT: jwtManager, Refresh: refreshTokenStore,
		StateTTL: 10 * time.Minute, StateSecret: stateSecret,
		PublicURL: publicOrigin,
	}

	router := api.NewRouter(api.Deps{
		Auth:          authH,
		VerifyEmail:   verifyH,
		TwoFA:         twoFAH,
		TwoFAVerify:   twoFAVerifyH,
		OAuthStart:    oauthStartH,
		OAuthCallback: oauthCallbackH,
		Legal:         &handler.LegalHandler{},
		UsersExport:   &handler.UsersExportHandler{Data: handler.NewUserDataAggregator(userStore, submissionStore)},
		CSRFSecret:    cfg.Auth.CSRFSecret,
		DevMail:       devMailH,
		Problem:       problemH,
		ProblemI18n:   problemI18nH,
		// Each section is gathered independently, and the error is carried out
		// in the SitemapSection rather than logged and dropped. Dropping it is
		// what previously turned a query against a non-existent column into a
		// clean-looking empty sitemap.
		Sitemap: func(ctx context.Context) []handler.SitemapSection {
			origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
			if origin == "" {
				origin = "http://localhost:8081"
			}
			toURLs := func(entries []postgres.SitemapEntry) []handler.SitemapURL {
				out := make([]handler.SitemapURL, 0, len(entries))
				for _, e := range entries {
					out = append(out, handler.SitemapURL{
						Location:   e.Location,
						LastMod:    e.LastMod,
						ChangeFreq: e.ChangeFreq,
						Priority:   e.Priority,
					})
				}
				return out
			}

			problems, errProblems := sitemapStore.PublicProblems(ctx, origin)
			contests, errContests := sitemapStore.PublicContests(ctx, origin)
			users, errUsers := sitemapStore.PublicUsers(ctx, origin)

			return []handler.SitemapSection{
				{Name: "problems", URLs: toURLs(problems), Err: errProblems},
				{Name: "contests", URLs: toURLs(contests), Err: errContests},
				{Name: "users", URLs: toURLs(users), Err: errUsers},
			}
		},
		// Feed items for /feed/problems.atom. The error is returned, not logged,
		// so a failed query surfaces as a 500 instead of a valid-looking empty
		// feed -- the mistake that hid the first sitemap's broken query.
		Feed: func(ctx context.Context) ([]handler.FeedItem, error) {
			origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
			if origin == "" {
				origin = "http://localhost:8081"
			}
			entries, err := feedStore.RecentProblems(ctx, 25)
			if err != nil {
				return nil, err
			}
			items := make([]handler.FeedItem, 0, len(entries))
			for _, e := range entries {
				url := origin + "/problems/" + e.Slug
				items = append(items, handler.FeedItem{
					Title:   e.Title,
					Link:    url,
					ID:      url,
					Summary: truncateForFeed(e.Summary, 500),
					Updated: toTime(e.Created),
				})
			}
			return items, nil
		},
		CommentFeed: func(ctx context.Context) ([]handler.FeedItem, error) {
			origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
			if origin == "" {
				origin = "http://localhost:8081"
			}
			entries, err := feedStore.RecentComments(ctx, 25)
			if err != nil {
				return nil, err
			}
			items := make([]handler.FeedItem, 0, len(entries))
			for _, e := range entries {
				// Link to the thread the comment belongs to, not the bare id.
				var link string
				switch e.ParentType {
				case "problem":
					link = origin + "/problems/" + strconv.FormatInt(e.ParentID, 10)
				case "contest":
					link = origin + "/contests/" + strconv.FormatInt(e.ParentID, 10)
				case "blog":
					link = origin + "/blog/" + strconv.FormatInt(e.ParentID, 10)
				default:
					link = origin + "/"
				}
				items = append(items, handler.FeedItem{
					Title:   e.Username + " commented",
					Link:    link,
					ID:      origin + "/comments/" + strconv.FormatInt(e.ID, 10),
					Summary: truncateForFeed(e.Content, 500),
					Updated: toTime(e.Created),
				})
			}
			return items, nil
		},
		BlogFeed: func(ctx context.Context) ([]handler.FeedItem, error) {
			origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
			if origin == "" {
				origin = "http://localhost:8081"
			}
			entries, err := feedStore.RecentBlogPosts(ctx, 25)
			if err != nil {
				return nil, err
			}
			items := make([]handler.FeedItem, 0, len(entries))
			for _, e := range entries {
				url := origin + "/blog/" + strconv.FormatInt(e.ID, 10)
				items = append(items, handler.FeedItem{
					Title:   e.Title,
					Link:    url,
					ID:      url,
					Summary: truncateForFeed(e.Content, 500),
					Updated: toTime(e.Created),
				})
			}
			return items, nil
		},
		ContestFeed: func(ctx context.Context) ([]handler.FeedItem, error) {
			origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
			if origin == "" {
				origin = "http://localhost:8081"
			}
			entries, err := feedStore.RecentContests(ctx, 25)
			if err != nil {
				return nil, err
			}
			items := make([]handler.FeedItem, 0, len(entries))
			for _, e := range entries {
				url := origin + "/contests/" + e.Slug
				items = append(items, handler.FeedItem{
					Title:   e.Title,
					Link:    url,
					ID:      url,
					Summary: truncateForFeed(e.Summary, 500),
					Updated: toTime(e.Created),
				})
			}
			return items, nil
		},
		Submission:     submissionH,
		Contest:        contestH,
		ContestProblem: contestProblemH,
		VJudge:         vjH,
		Admin:          adminH,
		Testcase:       testcaseH,
		WS:             wsManager,
		Rating:         ratingH,
		Registration:   registrationH,
		Virtual:        virtualH,
		Gym:            gymH,
		Hack:           hackH,
		Stats:          statsH,
		Notification:   notifH,
		Group:          groupH,
		Team:           teamH,
		Blog:           blogH,
		Editorial:      editorialH,
		APIKey:         apiKeyH,
		Webhook:        webhookH,
		Recommendation: recommendationH,
		Rankings:       rankingsH,
		Users:          usersH,
		Search:         searchH,
		LangLimit:      langLimitH,
		Import:         importH,
		Org:            orgH,
		Class:          classH,
		Training:       trainingH,
		Plagiarism:     plagiarismH,
		Media:          mediaH,
		Onsite:         onsiteH,
		OnsiteBatch:    onsiteBatchH,
		Clarification:  clarificationH,
		Notice:         noticeH,
		BotAccount:     botAccountH,
		Settings:       settingsH,
		LangAdmin:      langAdminH,
		RemoteLang:     remoteLangH,
		AdminSub:       adminSubH,
		Backup:         backupH,
		Generate:       generateH,
		AIModel:        aiModelH,
	}, jwtManager)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx)
	judgeQueue.Close()
}

// truncateForFeed bounds a problem statement for feed readers, which often
// display the whole summary inline. Matches dmoj's feed, which truncates the
// rendered description to 500 characters.
func truncateForFeed(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Trim on a rune boundary so a multi-byte character is never cut in half.
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

func toTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case *time.Time:
		if t != nil {
			return *t
		}
	}
	return time.Time{}
}
