// Package testdb opens the Postgres every integration test shares.
//
// It exists for one reason: the openers used to live in non-test files
// (`internal/store/postgres/testing.go`), so the `testing` package was linked
// into the production binary — a test framework shipped to customers, and a
// startup cost paid on every boot for a function only `go test` ever called.
// A `_test.go` file cannot be imported across packages, so the shared opener has
// to live in a real package; this is that package, and nothing in `cmd/` may
// import it.
//
// The skip-versus-fail decision lives in Require, and it is the important part:
// a missing database must FAIL in CI. Skipped tests report `ok`, so opening this
// file's history and seeing green CI told you nothing about whether the store
// layer — 58 files here, the largest block in the repo — was ever executed.
package testdb

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" driver
)

// DefaultDSN is what the openers fall back to when AIOJ_TEST_DSN is unset.
//
// It uses aioj_dev, the same password config.yaml and the Makefile's
// MIGRATE_DSN use, so `docker compose up -d postgres` plus a bare `go test`
// works with no environment at all. It used to say aioj_secret, which matched
// nothing else in the repo: `cmd/migrate` would migrate one database while the
// tests read another, and the mismatch only surfaced in CI, where the two
// tools finally pointed at the same host.
const DefaultDSN = "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"

// requiredEnv is set to "true" by CI. When it is set, an unreachable database is
// a test failure rather than a skip: the whole point of the workflow adding a
// Postgres service is that the store tests actually run, and a silent skip
// would let the service be dropped again without anyone noticing.
const requiredEnv = "AIOJ_TEST_DB_REQUIRED"

// DSN returns the test database URL, AIOJ_TEST_DSN or the local default.
func DSN() string {
	if dsn := os.Getenv("AIOJ_TEST_DSN"); dsn != "" {
		return dsn
	}
	return DefaultDSN
}

// Require opens a migrated Postgres for an integration test.
//
// Locally it skips when nothing is listening, so `make test` does not demand a
// running database. In CI (AIOJ_TEST_DB_REQUIRED=true) it fails loudly instead,
// because a skip there is indistinguishable from a pass in the job summary.
func Require(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", DSN())
	if err != nil {
		unavailable(t, "open", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		unavailable(t, "connect", err)
	}

	// Register the close so a leaking test cannot hold a connection pool open
	// for the rest of the package run.
	t.Cleanup(func() { db.Close() })
	return db
}

// unavailable skips or fails depending on whether the database was required.
func unavailable(t *testing.T, op string, err error) {
	t.Helper()

	if required() {
		t.Fatalf("test database is required (%s=true) but %s failed: %v\n"+
			"DSN: %s", requiredEnv, op, err, redact(DSN()))
	}
	t.Skipf("postgres unavailable (%s: %v); set AIOJ_TEST_DSN or run "+
		"`docker compose up -d postgres`, or set %s=true to require it",
		op, err, requiredEnv)
}

// required reports whether an unreachable database should fail the run.
func required() bool {
	v := os.Getenv(requiredEnv)
	return v == "1" || v == "true" || v == "TRUE"
}

// redact strips the password from a DSN so a failure message can name the host
// it tried without printing a credential into a CI log.
func redact(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return dsn
	}
	// Only look for the password separator after the scheme, so
	// "postgres://user:pass@host" loses the password and keeps the user.
	start := strings.Index(dsn, "://")
	if start < 0 {
		return dsn
	}
	start += 3
	colon := strings.Index(dsn[start:at], ":")
	if colon < 0 {
		return dsn
	}
	return dsn[:start+colon] + ":***" + dsn[at:]
}
