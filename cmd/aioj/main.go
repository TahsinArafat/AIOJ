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
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
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

	accessTTL, _ := time.ParseDuration(cfg.Auth.AccessTTL)
	refreshTTL, _ := time.ParseDuration(cfg.Auth.RefreshTTL)
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, accessTTL, refreshTTL)

	userStore := postgres.NewUserStore(db)
	refreshTokenStore := postgres.NewRefreshTokenStore(db)
	problemStore := postgres.NewProblemStore(db)
	submissionStore := postgres.NewSubmissionStore(db)

	judgeQueue := queue.NewMemory()
	execClient := executor.NewClient(cfg.Judge.Endpoint)
	workerPool := judge.NewWorkerPool(judgeQueue, execClient, cfg.LangDir, cfg.Judge.Concurrency, submissionStore, problemStore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go workerPool.Start(ctx)

	wsManager := handler.NewWSManager()
	authH := handler.NewAuthHandler(userStore, refreshTokenStore, jwtManager)
	problemH := handler.NewProblemHandler(problemStore)
	submissionH := handler.NewSubmissionHandler(submissionStore, problemStore, judgeQueue, wsManager)

	router := api.NewRouter(authH, problemH, submissionH, wsManager, jwtManager)

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
