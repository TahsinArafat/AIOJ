package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestLoad_DBPortFromEnv is the regression test for DB_PORT being ignored.
// compose and the docs both pass DB_PORT, but Load never read it, so the value
// in config.yaml (5432) always won — a database on any other port was simply
// unreachable, with no error pointing at the cause.
func TestLoad_DBPortFromEnv(t *testing.T) {
	path := writeTempConfig(t, `
environment: production
database:
  host: localhost
  port: 5432
  user: aioj
  password: x
  name: aioj
`)

	t.Setenv("DB_PORT", "6543")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Database.Port != 6543 {
		t.Fatalf("database.port = %d, want 6543 from DB_PORT", cfg.Database.Port)
	}
}

// TestLoad_DBPortIgnoresGarbage keeps a typo from replacing a working default
// with port 0.
func TestLoad_DBPortIgnoresGarbage(t *testing.T) {
	path := writeTempConfig(t, `
environment: production
database:
  host: localhost
  port: 5432
  user: aioj
  password: x
  name: aioj
`)

	for _, v := range []string{"not-a-port", "0", "-1"} {
		t.Setenv("DB_PORT", v)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if cfg.Database.Port != 5432 {
			t.Errorf("DB_PORT=%q changed the port to %d, want the configured 5432", v, cfg.Database.Port)
		}
	}
}
