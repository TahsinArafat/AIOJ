package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tahsinarafat/aioj/internal/seed"
)

// Thin shell: no logic. Reads env, calls seed.Seed, prints a one-line JSON
// summary, exits non-zero on error so `docker compose run` surfaces it.
func main() {
	cfg := seed.Config{
		BackendURL:  envOr("BACKEND_URL", "http://backend:8080"),
		AdminUser:   envOr("SEED_ADMIN_USERNAME", "ai"),
		AdminPass:   envOr("SEED_ADMIN_PASSWORD", "aiseedpass"),
		ProblemSlug: envOr("SEED_PROBLEM_SLUG", "hello"),
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
