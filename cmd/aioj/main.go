package main

import (
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/tahsinarafat/aioj/internal/config"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("starting aioj", "port", cfg.Server.Port)
}
