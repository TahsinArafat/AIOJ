package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// openTestDB opens a real Postgres for integration tests and skips the test
// when the database is unreachable (e.g. CI without a DB service).
//
// DSN override: AIOJ_TEST_DSN. CI should provide a migrated database; local
// runs may use the compose network address when the host port is overridden.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AIOJ_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://aioj:aioj_secret@localhost:5432/aioj?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("postgres unavailable (set AIOJ_TEST_DSN or start docker): %v", err)
	}
	return db
}
