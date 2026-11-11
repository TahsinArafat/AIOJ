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

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/api"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/authmethod"
	"github.com/tahsinarafat/aioj/internal/config"
	"github.com/tahsinarafat/aioj/internal/feature"
	"github.com/tahsinarafat/aioj/internal/generate"
	"github.com/tahsinarafat/aioj/internal/hack"
	"github.com/tahsinarafat/aioj/internal/harness"
	"github.com/tahsinarafat/aioj/internal/harness/problemgen"
	"github.com/tahsinarafat/aioj/internal/judge"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/judge/pressure"
	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/notify"
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
	mode := flag.String("mode", "server", "run mode: server|judge-worker|ai-worker|ai-harness")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("starting aioj", "port", cfg.Server.Port)

	// Refuse to boot with an empty, short, or published-default JWT secret:
	// anyone who reads this repo could otherwise forge an admin token.
	warnings, err := cfg.Validate()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	for _, w := range warnings {
		slog.Warn("insecure configuration", "detail", w)
	}

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
	problemMetaStore := postgres.NewProblemMetaStore(db)
	feedStore := postgres.NewFeedStore(db)
	submissionStore := postgres.NewSubmissionStore(db)
	contestStore := postgres.NewContestStore(db)
	ratingStore := postgres.NewRatingStore(db)

	var judgeQueue queue.JudgeQueue = queue.NewMemory()
	var redisClient *redis.Client
	if cfg.Redis.URL != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr(),
			Password: cfg.Redis.ResolvePassword(),
		})
		judgeQueue = queue.NewRedisQueue(redisClient)
		slog.Info("using redis judge queue", "url", cfg.Redis.URL)
	}
	execClient := executor.NewClient(cfg.Judge.Endpoint)
	langLimitStore := postgres.NewLanguageLimitStore(db)
	langLimitH := handler.NewLanguageLimitHandler(langLimitStore, problemStore)
	balloonStore := postgres.NewBalloonStore(db)
	printStore := postgres.NewPrintStore(db)
	workerPool := judge.NewWorkerPool(judgeQueue, execClient, cfg.LangDir, cfg.Judge.Concurrency, submissionStore, problemStore, langLimitStore, balloonStore)

	// Attach a pressure gate when thresholds are configured.  On Linux the
	// gate reads /proc/meminfo + /proc/loadavg before every dispatch; on
	// non-Linux hosts (dev, CI) DefaultGate uses a no-op sampler so the gate
	// is always transparent.
	pressureThresholds := pressure.Thresholds{
		MinFreeMemMB:  cfg.Judge.MinFreeMemMB,
		MaxLoadPerCPU: cfg.Judge.MaxLoadPerCPU,
	}
	if !pressureThresholds.Disabled() {
		workerPool.WithPressureGate(pressure.DefaultGate(pressureThresholds))
		slog.Info("judge pressure gate enabled",
			"min_free_mem_mb", pressureThresholds.MinFreeMemMB,
			"max_load_per_cpu", pressureThresholds.MaxLoadPerCPU,
		)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *mode == "judge-worker" {
		// Only dedicated judge processes consume the shared queue. The API
		// server must not silently become a third decider.
		reclaimer := judge.NewReclaimer(submissionStore, judgeQueue, judge.DefaultStaleClaimAfter, judge.DefaultReclaimInterval)
		go reclaimer.Start(ctx)
		go workerPool.Start(ctx)

		slog.Info("running in judge-worker mode, waiting for submissions")
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("judge-worker shutting down")
		cancel()
		// Drain before returning: cancelling alone abandons in-flight judgments
		// mid-write, and the old code recorded a System Error over them.
		workerPool.Stop()
		judgeQueue.Close()
		return
	}

	wsManager := handler.NewWSManager(jwtManager)
	// In server mode the same process also owns the judge workers, so a
	// finished verdict can be pushed straight to the subscribed browser. In
	// --mode=judge-worker there is no WS server here; the API process is
	// separate and the client falls back to polling.
	workerPool.WithNotifier(wsManager)
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
	// The refresh-token store is passed so a ban or an admin password reset ends
	// the target's sessions immediately instead of waiting out the 15m access TTL.
	adminH := handler.NewAdminHandler(userStore, setterStore, refreshTokenStore)

	testcaseH := handler.NewTestcaseHandler(problemStore, "./testdata")
	importH := handler.NewImportHandler(problemStore, "./testdata")
	ratingH := handler.NewRatingHandler(ratingStore, contestStore)
	registrationStore := postgres.NewRegistrationStore(db)
	registrationH := handler.NewRegistrationHandler(registrationStore, contestStore)
	virtualStore := postgres.NewVirtualStore(db)
	virtualService := virtual.NewService(virtualStore)
	virtualH := handler.NewVirtualHandler(virtualService, virtualStore, contestStore)
	gymStore := postgres.NewGymStore(db)
	gymH := handler.NewGymHandler(gymStore, contestStore)
	hackStore := postgres.NewHackStore(db)
	hackService := hack.NewService(hackStore, contestStore, submissionStore).
		WithJudge(problemStore, hack.NewJudge(execClient, cfg.LangDir))
	hackH := handler.NewHackHandler(hackService, hackStore, contestStore)
	statsH := handler.NewStatsHandler(submissionStore, problemStore)
	notifStore := postgres.NewNotificationStore(db)
	notifH := handler.NewNotificationHandler(notifStore)
	// The producer side of notifications: nothing wrote to the table before this,
	// so the bell was permanently empty. Attached to the handlers that have a
	// verdict to deliver.
	notifier := notify.New(notifStore)
	problemH.WithNotifier(notifier)
	contestH.WithNotifier(notifier)
	groupStore := postgres.NewGroupStore(db)
	groupH := handler.NewGroupHandler(groupStore, userStore, contestStore)
	teamH := handler.NewTeamHandler(teamStore, userStore)
	blogStore := postgres.NewBlogStore(db)
	blogH := handler.NewBlogHandler(blogStore)
	editorialStore := postgres.NewEditorialStore(db)
	editorialH := handler.NewEditorialHandler(editorialStore, problemStore, contestStore)
	apiKeyStore := postgres.NewAPIKeyStore(db)
	apiKeyH := handler.NewAPIKeyHandler(apiKeyStore)
	webhookStore := postgres.NewWebhookStore(db)
	webhookH := handler.NewWebhookHandler(webhookStore)
	recommendationH := handler.NewRecommendationHandler(problemStore, ratingStore)
	rankingsH := handler.NewRankingsHandler(userStore)
	usersH := handler.NewUsersHandler(userStore, blogStore, submissionStore, teamStore, groupStore, refreshTokenStore)
	searchStore := postgres.NewSearchStore(db)
	searchH := handler.NewSearchHandler(searchStore)

	orgStore := postgres.NewOrganizationStore(db)
	classStore := postgres.NewClassStore(db)
	trainingStore := postgres.NewTrainingPlanStore(db)

	orgH := handler.NewOrganizationHandler(orgStore)
	classH := handler.NewClassHandler(classStore, orgStore)
	trainingH := handler.NewTrainingHandler(trainingStore, orgStore)

	plagiarismStore := postgres.NewPlagiarismStore(db)
	// The similarity scan is the one CPU-bound job on the platform, so it can run
	// in a Rust process instead of Go. Selection is by environment (see
	// plagiarism.SelectEngine): "auto" uses the Rust binary when present and the
	// pure-Go engine otherwise, so a checkout without a Rust toolchain still boots.
	plagiarismEngine, err := plagiarism.SelectEngine()
	if err != nil {
		log.Fatalf("plagiarism engine: %v", err)
	}
	plagiarismService := plagiarism.NewServiceWithEngine(plagiarismStore, contestStore, submissionStore, plagiarismEngine)
	plagiarismH := handler.NewPlagiarismHandler(plagiarismService, plagiarismStore, contestStore)

	mediaH := handler.NewMediaHandler("./media")
	onsiteH := handler.NewOnsiteHandler(balloonStore, printStore, contestStore)
	onsiteBatchH := handler.NewOnsiteBatchHandler(contestStore, onsiteUserStore, userStore, refreshTokenStore, totpStore, jwtManager)

	clarificationStore := postgres.NewClarificationStore(db)
	clarificationH := handler.NewClarificationHandler(clarificationStore, contestStore)
	noticeStore := postgres.NewContestNoticeStore(db)
	noticeH := handler.NewContestNoticeHandler(noticeStore, contestStore)

	botAccountH := handler.NewAdminBotAccountHandler(botAccountStore, vjService)
	settingsStore := postgres.NewSystemSettingsStore(db)
	settingsH := handler.NewAdminSystemSettingsHandler(settingsStore)

	// One resolver is shared by the gates and the admin handler, so a policy
	// saved through the panel invalidates the cache the gates read.
	featureResolver := feature.NewResolver(settingsStore, userStore)
	featuresH := handler.NewFeaturesHandler(featureResolver, settingsStore)
	langAdminH := handler.NewAdminLanguageHandler(cfg.LangDir)

	remoteLangH := handler.NewRemoteLanguageHandler(remoteLangStore, vjService)
	adminSubH := handler.NewAdminSubmissionHandler(submissionStore, problemStore, vjService).
		WithQueue(judgeQueue).
		WithContestStore(contestStore)
	backupH := handler.NewAdminBackupHandler(userStore, cfg.Database, "./backups")

	credentialKey, err := ai.ParseCredentialKey(cfg.AI.CredentialMasterKey)
	if err != nil {
		log.Fatalf("AI credential key: %v", err)
	}
	var credentialBox *ai.CredentialBox
	if len(credentialKey) > 0 {
		credentialBox, err = ai.NewCredentialBox(credentialKey)
		if err != nil {
			log.Fatalf("AI credential encryption: %v", err)
		}
	}
	aiModelStore := postgres.NewAIModelStore(db, credentialBox)
	if cfg.AI.Enabled && len(credentialKey) == 0 {
		log.Fatal("AI is enabled but AI_CREDENTIAL_MASTER_KEY is not configured")
	}
	if err := aiModelStore.EncryptLegacyCredentials(ctx); err != nil {
		log.Fatalf("AI credential migration: %v", err)
	}
	aiModelH := handler.NewAdminAIModelHandler(aiModelStore)
	aiGenerationStore := postgres.NewAIGenerationStore(db)
	aiJobService := &generate.JobService{Jobs: aiGenerationStore, Models: aiModelStore, Problems: problemStore}
	generationAssetStore := postgres.NewGenerationAssetStore(db)
	aiWorker := &generate.JobWorker{Jobs: aiGenerationStore, Models: aiModelStore, Timeout: time.Duration(cfg.AI.JobTimeoutSeconds) * time.Second, Sandbox: &generate.SandboxValidator{Exec: execClient, LangDir: cfg.LangDir, StagingRoot: cfg.AI.StagedTestdataRoot}}
	startAIWorkers := func() {
		for i := 0; i < cfg.AI.WorkerConcurrency; i++ {
			go aiWorker.Run(ctx)
		}
	}
	// Retention: AI_RETENTION_DAYS was loaded and defaulted but never read, so
	// every generation job (prompt, proposal, validation transcript) was kept
	// forever. Purge terminal jobs older than the configured window. Runs in
	// every mode that touches the table; the first sweep is immediate so a
	// restart also cleans up.
	startAIRetention := func() {
		go func() {
			purge := func() {
				cutoff := time.Now().AddDate(0, 0, -cfg.AI.RetentionDays)
				if n, err := aiGenerationStore.PurgeCompletedBefore(ctx, cutoff); err != nil {
					if ctx.Err() == nil {
						slog.Error("ai retention purge failed", "error", err)
					}
				} else if n > 0 {
					slog.Info("ai retention purge", "deleted", n, "older_than_days", cfg.AI.RetentionDays)
				}
			}
			purge()
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					purge()
				}
			}
		}()
	}
	if *mode == "ai-worker" {
		if !cfg.AI.Enabled {
			log.Fatal("ai-worker mode requires AI_ENABLED=true")
		}
		startAIWorkers()
		startAIRetention()
		slog.Info("running in ai-worker mode, waiting for generation jobs")
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		cancel()
		return
	}

	// The harness runs the multi-turn generator pipeline: the model calls
	// tools, the tools materialize test data by executing generated code inside
	// go-judge. It is a separate mode (and, in the shipped compose file, a
	// separate container) so a long, token-heavy run cannot compete with API
	// traffic. AI_HARNESS_ENABLED gates it.
	if *mode == "ai-harness" {
		if !cfg.AI.Enabled || !cfg.AI.Harness {
			log.Fatal("ai-harness mode requires AI_ENABLED=true and AI_HARNESS_ENABLED=true")
		}
		problemgen.MaxN = cfg.AI.MaxCaseN
		runner, err := problemgen.NewJudgeRunner(execClient, cfg.LangDir)
		if err != nil {
			log.Fatalf("ai harness: %v", err)
		}
		harnessWorker := &problemgen.Worker{
			Jobs:       aiGenerationStore,
			Runner:     runner,
			Corpus:     postgres.NewNoveltyCorpus(db),
			Problems:   problemStore,
			Transcript: postgres.NewHarnessTranscript(db),
			Staging:    cfg.AI.StagedTestdataRoot,
			MaxBytes:   cfg.AI.MaxJobBytes,
			Budget:     problemgen.DefaultBudget(cfg.AI.RepairTurns),
			Resolve: func(ctx context.Context, modelID *string) (harness.Model, error) {
				m, err := aiModelStore.GetDefault(ctx)
				if modelID != nil {
					m, err = aiModelStore.GetByID(ctx, *modelID)
				}
				if err != nil || m == nil {
					return nil, fmt.Errorf("no AI model is enabled")
				}
				return ai.NewWithOptions(m.Endpoint, m.APIKey, m.ModelName, ai.ClientOptions{
					Mode:                m.StructuredOutputMode,
					Timeout:             time.Duration(m.RequestTimeoutSeconds) * time.Second,
					MaxOutputTokens:     m.MaxOutputTokens,
					AllowPrivateNetwork: m.AllowPrivateNetwork,
				}), nil
			},
		}
		if err := harnessWorker.Ready(); err != nil {
			log.Fatalf("ai harness: %v", err)
		}
		go harnessWorker.Run(ctx)
		slog.Info("running in ai-harness mode, waiting for generation jobs")
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		cancel()
		return
	}
	// Development/default deployment: process jobs in the API process. A
	// dedicated --mode=ai-worker can be used to isolate workloads.
	//
	// Skipped when the harness is enabled: problemgen.Worker (--mode=ai-harness)
	// claims from the same ai_generation_jobs queue, so leaving the single-shot
	// worker running would race it for every job. Whichever calls ClaimNext
	// first wins, which made a harness deployment silently fall back to the
	// legacy one-shot pipeline on a coin flip. The harness container owns the
	// queue whenever it is deployed.
	if cfg.AI.Enabled && !cfg.AI.Harness {
		startAIWorkers()
	}
	// Retention runs regardless of AI_ENABLED: old jobs left from a period when
	// generation was on must still be purged after it is switched off.
	startAIRetention()

	verifyH := &handler.EmailVerificationHandler{Users: userStore, Tokens: evtStore}
	// The mail-catcher inspector exposes every message (verification and
	// password-reset links) to anyone who can reach the API. Mount it only in
	// development — never in production, even when the driver is "catcher".
	var devMailH *handler.DevMailHandler
	if cfg.Mail.Driver == "catcher" && cfg.IsDevelopment() {
		devMailH = &handler.DevMailHandler{Sender: mailSender}
		slog.Warn("dev mail inspector mounted at /api/dev/mail (environment=development)")
	} else if cfg.Mail.Driver == "catcher" {
		slog.Warn("mail.driver=catcher outside development: messages are captured in memory " +
			"and /api/dev/mail stays disabled")
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
	// Sign-in methods. "Configured" is what this deployment can actually do
	// (an OAuth provider with no client id cannot work), and it is read from
	// the providers themselves rather than from the raw config, so the two can
	// never disagree.
	authMethods := authmethod.NewResolver(settingsStore, authmethod.Configured{
		Google: oauthProviders["google"].Config().ClientID != "",
		GitHub: oauthProviders["github"].Config().ClientID != "",
	})
	authMethodsH := &handler.AuthMethodsHandler{Methods: authMethods}
	authH.SetAuthMethods(authMethods)

	oauthStartH := &handler.OAuthStartHandler{
		Providers: oauthProviders, StateSecret: stateSecret, StateTTL: 10 * time.Minute,
		Methods: authMethods,
	}
	oauthCallbackH := &handler.OAuthCallbackHandler{
		Users: userStore, Links: oauthLinkStore, Providers: oauthProviders,
		JWT: jwtManager, Refresh: refreshTokenStore,
		StateTTL: 10 * time.Minute, StateSecret: stateSecret,
		PublicURL: publicOrigin,
	}

	auditStore := postgres.NewAuditStore(db)
	aiApplyService := &generate.ApplyService{Jobs: aiGenerationStore, Problems: problemStore, Editorials: editorialStore, Assets: generationAssetStore, Audit: auditStore, TestdataRoot: "./testdata", StagingRoot: cfg.AI.StagedTestdataRoot}
	aiGenerationH := handler.NewAIGenerationHandler(aiJobService, aiApplyService, aiGenerationStore, aiModelStore, problemStore)
	healthH := &handler.HealthChecker{
		DB:      db,
		Started: time.Now(),
	}
	if redisClient != nil {
		healthH.RedisPinger = func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }
	}
	healthH.QueueDepth = judgeQueue.Len
	auditH := handler.NewAuditLogHandler(auditStore)
	// Setters can rejudge their own problem's submissions, so those actions
	// have to land in the same audit trail as the admin endpoints.
	submissionH.WithAudit(auditH)

	router := api.NewRouter(api.Deps{
		Auth:          authH,
		VerifyEmail:   verifyH,
		TwoFA:         twoFAH,
		TwoFAVerify:   twoFAVerifyH,
		OAuthStart:    oauthStartH,
		OAuthCallback: oauthCallbackH,
		AuthMethods:   authMethodsH,
		Legal:         &handler.LegalHandler{},
		UsersExport:   &handler.UsersExportHandler{Data: handler.NewUserDataAggregator(userStore, submissionStore)},
		UsersDeletion: &handler.UsersDeletionHandler{Users: userStore, Deleter: handler.NewUserCascadeDeleter(db)},
		CSRFSecret:    cfg.Auth.CSRFSecret,
		DevMail:       devMailH,
		Problem:       problemH,
		ProblemI18n:   problemI18nH,
		// Link-preview cards for /meta/<path>. Social crawlers do not run
		// JavaScript, so the SPA's client-set title never reaches them — this is
		// what gives a shared problem link its own title, description and image.
		Meta: &handler.MetaHandler{
			Origin: func() string { return os.Getenv("PUBLIC_ORIGIN") },
			ProblemCard: func(ctx context.Context, slug string) (*handler.ShareCard, error) {
				m, err := problemMetaStore.GetBySlug(ctx, slug)
				if err != nil {
					return nil, err
				}
				// Private problems stay invisible to crawlers: a nil card makes
				// the handler answer 404 rather than leak the title.
				if m == nil || !m.Visible {
					return nil, nil
				}
				return &handler.ShareCard{
					Title:       m.Title,
					Description: m.Description,
					Path:        "/problems/" + m.Slug,
					Kind:        "article",
					Facts:       shareCardFacts(m),
				}, nil
			},
		},
		// Each section is gathered independently, and the error is carried out
		// in the SitemapSection rather than logged and dropped. Dropping it is
		// what previously turned a query against a non-existent column into a
		// clean-looking empty sitemap.
		//
		// The sitemap is cached in-process for 1 hour: crawlers hit /sitemap.xml
		// frequently and the content changes at most a few times per day.  A
		// transient DB error returns the stale cached copy (if any) instead of
		// serving a 500 to the crawler.
		Sitemap: func() func(ctx context.Context) []handler.SitemapSection {
			cache := &handler.TTLCache[[]handler.SitemapSection]{TTL: time.Hour}
			return func(ctx context.Context) []handler.SitemapSection {
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
				sections, _ := cache.Get(func() ([]handler.SitemapSection, error) {
					problems, errProblems := sitemapStore.PublicProblems(ctx, origin)
					contests, errContests := sitemapStore.PublicContests(ctx, origin)
					users, errUsers := sitemapStore.PublicUsers(ctx, origin)
					// If any section errored, propagate so the caller can 500
					// rather than serve a silently incomplete sitemap.
					for _, e := range []error{errProblems, errContests, errUsers} {
						if e != nil {
							return nil, e
						}
					}
					return []handler.SitemapSection{
						{Name: "problems", URLs: toURLs(problems)},
						{Name: "contests", URLs: toURLs(contests)},
						{Name: "users", URLs: toURLs(users)},
						// Static pages: always present, no store to fail.
						{Name: "legal", URLs: handler.LegalSitemapURLs(origin)},
					}, nil
				})
				return sections
			}
		}(),
		// Feed items for /feed/problems.atom. The error is returned, not logged,
		// so a failed query surfaces as a 500 instead of a valid-looking empty
		// feed -- the mistake that hid the first sitemap's broken query.
		// Cached for 10 minutes: feeds are polled by RSS readers frequently.
		Feed: func() func(ctx context.Context) ([]handler.FeedItem, error) {
			cache := &handler.TTLCache[[]handler.FeedItem]{TTL: 10 * time.Minute}
			return func(ctx context.Context) ([]handler.FeedItem, error) {
				origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
				if origin == "" {
					origin = "http://localhost:8081"
				}
				return cache.Get(func() ([]handler.FeedItem, error) {
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
				})
			}
		}(),
		CommentFeed: func() func(ctx context.Context) ([]handler.FeedItem, error) {
			cache := &handler.TTLCache[[]handler.FeedItem]{TTL: 10 * time.Minute}
			return func(ctx context.Context) ([]handler.FeedItem, error) {
				origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
				if origin == "" {
					origin = "http://localhost:8081"
				}
				return cache.Get(func() ([]handler.FeedItem, error) {
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
				})
			}
		}(),
		BlogFeed: func() func(ctx context.Context) ([]handler.FeedItem, error) {
			cache := &handler.TTLCache[[]handler.FeedItem]{TTL: 10 * time.Minute}
			return func(ctx context.Context) ([]handler.FeedItem, error) {
				origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
				if origin == "" {
					origin = "http://localhost:8081"
				}
				return cache.Get(func() ([]handler.FeedItem, error) {
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
				})
			}
		}(),
		ContestFeed: func() func(ctx context.Context) ([]handler.FeedItem, error) {
			cache := &handler.TTLCache[[]handler.FeedItem]{TTL: 10 * time.Minute}
			return func(ctx context.Context) ([]handler.FeedItem, error) {
				origin := handler.NormalizeOrigin(os.Getenv("PUBLIC_ORIGIN"))
				if origin == "" {
					origin = "http://localhost:8081"
				}
				return cache.Get(func() ([]handler.FeedItem, error) {
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
				})
			}
		}(),
		Submission:      submissionH,
		Contest:         contestH,
		ContestProblem:  contestProblemH,
		VJudge:          vjH,
		Admin:           adminH,
		Testcase:        testcaseH,
		WS:              wsManager,
		Rating:          ratingH,
		Registration:    registrationH,
		Virtual:         virtualH,
		Gym:             gymH,
		Hack:            hackH,
		Stats:           statsH,
		Notification:    notifH,
		Group:           groupH,
		Team:            teamH,
		Blog:            blogH,
		Editorial:       editorialH,
		APIKey:          apiKeyH,
		APIKeys:         apiKeyStore,
		APIKeyUsers:     userStore,
		Webhook:         webhookH,
		Recommendation:  recommendationH,
		Rankings:        rankingsH,
		Users:           usersH,
		Search:          searchH,
		LangLimit:       langLimitH,
		Import:          importH,
		Org:             orgH,
		Class:           classH,
		Training:        trainingH,
		Plagiarism:      plagiarismH,
		Media:           mediaH,
		Onsite:          onsiteH,
		OnsiteBatch:     onsiteBatchH,
		Clarification:   clarificationH,
		Notice:          noticeH,
		BotAccount:      botAccountH,
		Settings:        settingsH,
		LangAdmin:       langAdminH,
		RemoteLang:      remoteLangH,
		AdminSub:        adminSubH,
		Backup:          backupH,
		CDN:             handler.NewCDNHandler(handler.CDNConfig{}),
		Achievements:    handler.NewAchievementHandler(postgres.NewAchievementStore(db)),
		Friendships:     handler.NewFriendshipHandler(postgres.NewFriendshipStore(db), userStore),
		AuditLog:        auditH,
		Health:          healthH,
		Activity:        handler.NewActivityHandler(submissionStore, ratingStore, feedStore),
		AIModel:         aiModelH,
		AIGeneration:    aiGenerationH,
		Features:        featuresH,
		FeatureResolver: featureResolver,
		AIEnabled:       cfg.AI.Enabled,
		MetricsToken:    cfg.Server.MetricsToken,
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

// shareCardFacts renders the short labels under a problem's link preview:
// difficulty, limits and the first few tags. A preview body is a couple of
// lines, so this keeps to what a solver scanning a shared link cares about.
func shareCardFacts(m *postgres.ProblemMeta) []string {
	facts := make([]string, 0, 4)
	if d := strings.TrimSpace(m.Difficulty); d != "" {
		r := []rune(d)
		facts = append(facts, strings.ToUpper(string(r[0]))+string(r[1:]))
	}
	if m.TimeLimit > 0 {
		facts = append(facts, fmt.Sprintf("%.1f s", float64(m.TimeLimit)/1000))
	}
	if m.MemoryLimit > 0 {
		facts = append(facts, fmt.Sprintf("%d MB", m.MemoryLimit))
	}
	if len(m.Tags) > 0 {
		tags := m.Tags
		if len(tags) > 4 {
			tags = tags[:4]
		}
		facts = append(facts, strings.Join(tags, ", "))
	}
	return facts
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
