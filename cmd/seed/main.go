package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tahsinarafat/aioj/internal/seed"
)

// Thin shell: no logic. Reads env, calls seed.Seed, prints a one-line JSON
// summary, exits non-zero on error so `docker compose run` surfaces it.
func main() {
	// AIOJ_ENV defaults to production, matching the server. Only a development
	// environment may fall back to the published demo password.
	dev := strings.EqualFold(strings.TrimSpace(envOr("AIOJ_ENV", "")), "development")

	cfg := seed.Config{
		BackendURL:       envOr("BACKEND_URL", "http://backend:8080"),
		AdminUser:        envOr("SEED_ADMIN_USERNAME", "ai"),
		AdminPass:        envOr("SEED_ADMIN_PASSWORD", ""),
		ProblemSlug:      envOr("SEED_PROBLEM_SLUG", "hello"),
		AllowDefaultPass: dev,
	}
	if dev && cfg.AdminPass == "" {
		fmt.Fprintln(os.Stderr, "seed: AIOJ_ENV=development — using the built-in demo admin password; "+
			"set SEED_ADMIN_PASSWORD to override")
	}

	res, err := seed.Seed(context.Background(), cfg)
	out, _ := json.Marshal(res)
	fmt.Printf(`{"seed":%s}`+"\n", out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
