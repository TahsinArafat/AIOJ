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

// TestLoad_RemoteJudgeDefaults covers the two submit-service addresses.
//
// They were read straight from the environment in main(), with the defaults
// inlined there. That left them out of the example config and out of Validate, so
// an operator had no way to discover them except by reading main.go. Load now
// resolves them, env first, then the same defaults main used.
func TestLoad_RemoteJudgeDefaults(t *testing.T) {
	path := writeTempConfig(t, `
environment: production
database:
  host: localhost
  port: 5432
  user: aioj
  password: x
  name: aioj
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RemoteJudge.CFSubmitURL != "http://host.docker.internal:8003" {
		t.Errorf("cf_submit_url = %q, want the default main() used", cfg.RemoteJudge.CFSubmitURL)
	}
	if cfg.RemoteJudge.AtCoderSubmitURL != "http://host.docker.internal:8004" {
		t.Errorf("atcoder_submit_url = %q, want the default main() used", cfg.RemoteJudge.AtCoderSubmitURL)
	}
}

func TestLoad_RemoteJudgeFromEnv(t *testing.T) {
	path := writeTempConfig(t, `
environment: production
database:
  host: localhost
  port: 5432
  user: aioj
  password: x
  name: aioj
`)
	t.Setenv("CF_SUBMIT_URL", "http://cf-submit:9000")
	t.Setenv("ATCODER_SUBMIT_URL", "http://atcoder-submit:9001")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RemoteJudge.CFSubmitURL != "http://cf-submit:9000" {
		t.Errorf("cf_submit_url = %q, want the CF_SUBMIT_URL value", cfg.RemoteJudge.CFSubmitURL)
	}
	if cfg.RemoteJudge.AtCoderSubmitURL != "http://atcoder-submit:9001" {
		t.Errorf("atcoder_submit_url = %q, want the ATCODER_SUBMIT_URL value", cfg.RemoteJudge.AtCoderSubmitURL)
	}
}

// TestLoad_FeedToken covers the ICPC feed credential. It had no default and no
// config key, so it could only be set by an env var nobody documented.
func TestLoad_FeedToken(t *testing.T) {
	path := writeTempConfig(t, `
environment: production
database:
  host: localhost
  port: 5432
  user: aioj
  password: x
  name: aioj
`)
	if cfg, err := Load(path); err != nil {
		t.Fatalf("load: %v", err)
	} else if cfg.Server.FeedToken != "" {
		t.Errorf("feed_token = %q, want empty when unset", cfg.Server.FeedToken)
	}

	t.Setenv("FEED_TOKEN", "scorer-secret")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.FeedToken != "scorer-secret" {
		t.Errorf("feed_token = %q, want the FEED_TOKEN value", cfg.Server.FeedToken)
	}
}

// TestPublicOrigin prefers the resolved mail value.
//
// main() re-read PUBLIC_ORIGIN at six call sites while Load had already resolved
// it, so setting public_url in yaml had no effect on the sitemap or the link
// previews. PublicOrigin is the one answer, and PUBLIC_ORIGIN is only the
// fallback Load applies when the yaml key is absent.
func TestPublicOrigin(t *testing.T) {
	path := writeTempConfig(t, `
environment: production
database:
  host: localhost
  port: 5432
  user: aioj
  password: x
  name: aioj
mail:
  public_url: https://aioj.example
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.PublicOrigin(); got != "https://aioj.example" {
		t.Errorf("PublicOrigin() = %q, want the yaml value", got)
	}
}
