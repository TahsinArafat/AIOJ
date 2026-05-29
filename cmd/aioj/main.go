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
	"syscall"
	"time"

	"github.com/tahsinarafat/aioj/internal/api"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/config"
	"github.com/tahsinarafat/aioj/internal/judge"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/hack"
	"github.com/tahsinarafat/aioj/internal/plagiarism"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"github.com/tahsinarafat/aioj/internal/virtual"
	"github.com/tahsinarafat/aioj/internal/vjudge"

	"github.com/redis/go-redis/v9"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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
	authH := handler.NewAuthHandler(userStore, refreshTokenStore, passwordResetTokenStore, jwtManager)
	problemH := handler.NewProblemHandler(problemStore)
	submissionH := handler.NewSubmissionHandler(submissionStore, problemStore, contestStore, judgeQueue, wsManager, execClient, cfg.LangDir)
	contestH := handler.NewContestHandler(contestStore, ratingStore)

	botAccountStore := postgres.NewBotAccountStore(db)
	vjService := vjudge.NewService(submissionStore, botAccountStore)

	platforms := []string{"codeforces", "atcoder", "cses", "toph", "qoj"}
	for _, platform := range platforms {
		accounts, _ := botAccountStore.ListByPlatform(ctx, platform)
		cfg := vjudge.BotConfig{}
		if len(accounts) > 0 && accounts[0].Status == "active" {
			cfg.Username = accounts[0].PlatformUser
			cfg.Password = accounts[0].PlatformPass
			cfg.APIKey = accounts[0].APIKey
			cfg.APISecret = accounts[0].APISecret
			cfg.Cookies = accounts[0].SessionData
		}
		switch platform {
		case "codeforces":
			vjService.RegisterBot(platform, vjudge.NewCodeforcesBot(cfg))
		case "atcoder":
			vjService.RegisterBot(platform, vjudge.NewAtCoderBot(cfg))
		case "cses":
			vjService.RegisterBot(platform, vjudge.NewCSESBot(cfg))
		case "toph":
			vjService.RegisterBot(platform, vjudge.NewTophBot(cfg))
		case "qoj":
			vjService.RegisterBot(platform, vjudge.NewQOJBot(cfg))
		}
	}

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
	groupH := handler.NewGroupHandler(groupStore)
	teamStore := postgres.NewTeamStore(db)
	teamH := handler.NewTeamHandler(teamStore)
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
	usersH := handler.NewUsersHandler(userStore)
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
	plagiarismH := handler.NewPlagiarismHandler(plagiarismService, plagiarismStore)

	mediaH := handler.NewMediaHandler("./media")
	onsiteH := handler.NewOnsiteHandler(balloonStore, printStore, contestStore)

	botAccountH := handler.NewAdminBotAccountHandler(botAccountStore)
	settingsStore := postgres.NewSystemSettingsStore(db)
	settingsH := handler.NewAdminSystemSettingsHandler(settingsStore)
	langAdminH := handler.NewAdminLanguageHandler(cfg.LangDir)

	router := api.NewRouter(authH, problemH, submissionH, contestH, vjH, adminH, testcaseH, wsManager, jwtManager, ratingH, registrationH, virtualH, gymH, hackH, statsH, notifH, groupH, teamH, blogH, editorialH, apiKeyH, webhookH, recommendationH, rankingsH, usersH, searchH, langLimitH, importH, orgH, classH, trainingH, plagiarismH, mediaH, onsiteH, botAccountH, settingsH, langAdminH)

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
