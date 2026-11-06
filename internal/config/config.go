package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Environment names. Anything other than EnvDevelopment is treated as
// production, so a missing or misspelled value fails closed.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

type Config struct {
	// Environment selects the deployment profile. Development relaxes a few
	// safety checks (weak secrets warn instead of aborting, dev-only routes are
	// mounted). Default is production.
	Environment string         `yaml:"environment"`
	Server      ServerConfig   `yaml:"server"`
	Database    DatabaseConfig `yaml:"database"`
	Auth        AuthConfig     `yaml:"auth"`
	OAuth       OAuthConfig    `yaml:"oauth"`
	Judge       JudgeConfig    `yaml:"judge"`
	Redis       RedisConfig    `yaml:"redis"`
	AI          AIConfig       `yaml:"ai"`
	Mail        MailConfig     `yaml:"mail"`
	Features    FeaturesConfig `yaml:"features"`
	LangDir     string         `yaml:"lang_dir"`
}

// IsDevelopment reports whether dev-only behaviour (mail catcher inspector,
// relaxed secret checks) is allowed.
func (c *Config) IsDevelopment() bool {
	return strings.EqualFold(strings.TrimSpace(c.Environment), EnvDevelopment)
}

// minJWTSecretLen is the shortest HS256 key we accept. Shorter keys are
// brute-forceable offline from a single captured token.
const minJWTSecretLen = 32

// knownWeakJWTSecrets are values that have shipped in docs, examples, or
// earlier revisions of this repo. Anyone can sign a token with them.
var knownWeakJWTSecrets = map[string]struct{}{
	"change-me-in-production":                          {},
	"change-me-in-production-use-a-long-random-string": {},
	"changeme":   {},
	"jwt-secret": {},
	"secret":     {},
	"dev":        {},
	"test":       {},
}

// Validate reports fatal configuration problems. In development, weak secrets
// are returned as warnings instead of errors so a local checkout still boots;
// in production they abort startup. An empty JWT secret is always fatal.
func (c *Config) Validate() (warnings []string, err error) {
	if msg := c.jwtSecretProblem(); msg != "" {
		if c.Auth.JWTSecret == "" || !c.IsDevelopment() {
			return nil, fmt.Errorf("auth.jwt_secret %s — set JWT_SECRET to a random value "+
				"(e.g. `openssl rand -hex 32`)", msg)
		}
		warnings = append(warnings, fmt.Sprintf("auth.jwt_secret %s — acceptable only because "+
			"environment=development; set JWT_SECRET before deploying", msg))
	}
	if !c.IsDevelopment() && strings.EqualFold(c.Mail.Driver, "catcher") {
		warnings = append(warnings, "mail.driver=catcher in production: outbound mail is "+
			"captured in memory and never delivered")
	}
	return warnings, nil
}

// jwtSecretProblem returns a human-readable description of what is wrong with
// the configured JWT secret, or "" when it is acceptable.
func (c *Config) jwtSecretProblem() string {
	secret := strings.TrimSpace(c.Auth.JWTSecret)
	switch {
	case secret == "":
		return "is empty"
	case len(secret) < minJWTSecretLen:
		return fmt.Sprintf("is too short (%d bytes, need at least %d)", len(secret), minJWTSecretLen)
	}
	if _, weak := knownWeakJWTSecrets[strings.ToLower(secret)]; weak {
		return "is a published default value"
	}
	if strings.Trim(secret, string(secret[0])) == "" {
		return "is a single repeated character"
	}
	return ""
}

// FeaturesConfig holds geolocation / rollout toggles.
// Remote bot platforms (CF, AtCoder, …) can be restricted by ISO country code
// so we do not hammer regions where they are blocked or rate-limited hard.
type FeaturesConfig struct {
	// RemoteBotsRegions is an allowlist of ISO 3166-1 alpha-2 codes.
	// Empty means "enabled everywhere".
	RemoteBotsRegions []string `yaml:"remote_bots_regions"`
}

// MailConfig selects the transactional mail backend and builds absolute links.
type MailConfig struct {
	Driver    string `yaml:"driver"` // "smtp" | "catcher" | "noop"
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	From      string `yaml:"from"`
	PublicURL string `yaml:"public_url"` // e.g. https://aioj.com (used in email links)
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	MaxOpen  int    `yaml:"max_open"`
	MaxIdle  int    `yaml:"max_idle"`
	// ReplicaDSN is an optional read-replica connection (Phase E).
	// When empty, all reads stay on the primary.
	ReplicaDSN string `yaml:"replica_dsn"`
}

type AuthConfig struct {
	CSRFSecret string `yaml:"csrf_secret"`
	JWTSecret  string `yaml:"jwt_secret"`
	AccessTTL  string `yaml:"access_ttl"`
	RefreshTTL string `yaml:"refresh_ttl"`
}

// OAuthConfig holds optional GitHub/Google SSO credentials.
type OAuthConfig struct {
	StateSecret string              `yaml:"state_secret"`
	GitHub      OAuthProviderConfig `yaml:"github"`
	Google      OAuthProviderConfig `yaml:"google"`
}

type OAuthProviderConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
}

type JudgeConfig struct {
	Endpoint    string `yaml:"endpoint"`
	Concurrency int    `yaml:"concurrency"`
	MaxCodeSize int64  `yaml:"max_code_size"`
}

type RedisConfig struct {
	URL string `yaml:"url"`
	// Password authenticates against a protected Redis (REDIS_PASSWORD). When
	// empty, a password embedded in URL (redis://:pass@host:port) is used.
	Password string `yaml:"password"`
}

// Addr returns the host:port go-redis expects, accepting either a bare
// "host:port" (the historical form) or a redis:// URL.
func (r RedisConfig) Addr() string {
	u, err := url.Parse(strings.TrimSpace(r.URL))
	if err != nil || u.Host == "" || (u.Scheme != "redis" && u.Scheme != "rediss") {
		return strings.TrimSpace(r.URL)
	}
	return u.Host
}

// ResolvePassword returns the configured password, falling back to credentials
// embedded in the URL.
func (r RedisConfig) ResolvePassword() string {
	if r.Password != "" {
		return r.Password
	}
	u, err := url.Parse(strings.TrimSpace(r.URL))
	if err != nil || u.User == nil {
		return ""
	}
	if p, ok := u.User.Password(); ok {
		return p
	}
	return ""
}

type AIConfig struct {
	Enabled             bool   `yaml:"enabled"`
	Endpoint            string `yaml:"endpoint"` // base URL of OpenAI-compatible API, e.g. http://localhost:8080/v1
	APIKey              string `yaml:"api_key"`
	Model               string `yaml:"model"` // model name served at the endpoint
	CredentialMasterKey string `yaml:"credential_master_key"`
	WorkerConcurrency   int    `yaml:"worker_concurrency"`
	JobTimeoutSeconds   int    `yaml:"job_timeout_seconds"`
	MaxResponseBytes    int64  `yaml:"max_response_bytes"`
	RetentionDays       int    `yaml:"retention_days"`
	StagedTestdataRoot  string `yaml:"staged_testdata_root"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if v := os.Getenv("AIOJ_ENV"); v != "" {
		cfg.Environment = strings.TrimSpace(v)
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		cfg.Server.Port = atoi(v)
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DATABASE_REPLICA_DSN"); v != "" {
		cfg.Database.ReplicaDSN = v
	}
	if v := os.Getenv("JUDGE_ENDPOINT"); v != "" {
		cfg.Judge.Endpoint = v
	}
	if v := os.Getenv("JUDGE_CONCURRENCY"); v != "" {
		cfg.Judge.Concurrency = atoi(v)
	}
	if v := os.Getenv("REDIS_URL"); v != "" {
		cfg.Redis.URL = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("AI_ENDPOINT"); v != "" {
		cfg.AI.Endpoint = v
	}
	if v := os.Getenv("AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}
	if v := os.Getenv("AI_MODEL"); v != "" {
		cfg.AI.Model = v
	}
	if v := os.Getenv("AI_CREDENTIAL_MASTER_KEY"); v != "" {
		cfg.AI.CredentialMasterKey = v
	}
	if v := os.Getenv("AI_WORKER_CONCURRENCY"); v != "" {
		cfg.AI.WorkerConcurrency = atoi(v)
	}
	if v := os.Getenv("AI_JOB_TIMEOUT_SECONDS"); v != "" {
		cfg.AI.JobTimeoutSeconds = atoi(v)
	}
	if v := os.Getenv("AI_MAX_RESPONSE_BYTES"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		cfg.AI.MaxResponseBytes = n
	}
	if v := os.Getenv("AI_RETENTION_DAYS"); v != "" {
		cfg.AI.RetentionDays = atoi(v)
	}
	if v := os.Getenv("AI_STAGED_TESTDATA_ROOT"); v != "" {
		cfg.AI.StagedTestdataRoot = v
	}
	if v := os.Getenv("AI_ENABLED"); v != "" {
		cfg.AI.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if cfg.AI.WorkerConcurrency <= 0 {
		cfg.AI.WorkerConcurrency = 2
	}
	if cfg.AI.JobTimeoutSeconds <= 0 {
		cfg.AI.JobTimeoutSeconds = 300
	}
	if cfg.AI.MaxResponseBytes <= 0 {
		cfg.AI.MaxResponseBytes = 4 << 20
	}
	if cfg.AI.RetentionDays <= 0 {
		cfg.AI.RetentionDays = 14
	}
	if cfg.AI.StagedTestdataRoot == "" {
		cfg.AI.StagedTestdataRoot = "./testdata/ai-staging"
	}
	if v := os.Getenv("CSRF_SECRET"); v != "" {
		cfg.Auth.CSRFSecret = v
	}
	if v := os.Getenv("OAUTH_STATE_SECRET"); v != "" {
		cfg.OAuth.StateSecret = v
	}
	if v := os.Getenv("GITHUB_CLIENT_ID"); v != "" {
		cfg.OAuth.GitHub.ClientID = v
	}
	if v := os.Getenv("GITHUB_CLIENT_SECRET"); v != "" {
		cfg.OAuth.GitHub.ClientSecret = v
	}
	if v := os.Getenv("GITHUB_REDIRECT_URL"); v != "" {
		cfg.OAuth.GitHub.RedirectURL = v
	}
	if v := os.Getenv("GOOGLE_CLIENT_ID"); v != "" {
		cfg.OAuth.Google.ClientID = v
	}
	if v := os.Getenv("GOOGLE_CLIENT_SECRET"); v != "" {
		cfg.OAuth.Google.ClientSecret = v
	}
	if v := os.Getenv("GOOGLE_REDIRECT_URL"); v != "" {
		cfg.OAuth.Google.RedirectURL = v
	}
	if v := os.Getenv("MAIL_DRIVER"); v != "" {
		cfg.Mail.Driver = v
	}
	if v := os.Getenv("MAIL_HOST"); v != "" {
		cfg.Mail.Host = v
	}
	if v := os.Getenv("MAIL_PORT"); v != "" {
		cfg.Mail.Port = atoi(v)
	}
	if v := os.Getenv("MAIL_USERNAME"); v != "" {
		cfg.Mail.Username = v
	}
	if v := os.Getenv("MAIL_PASSWORD"); v != "" {
		cfg.Mail.Password = v
	}
	if v := os.Getenv("MAIL_FROM"); v != "" {
		cfg.Mail.From = v
	}
	if v := os.Getenv("MAIL_PUBLIC_URL"); v != "" {
		cfg.Mail.PublicURL = v
	}
	if v := os.Getenv("PUBLIC_ORIGIN"); v != "" && cfg.Mail.PublicURL == "" {
		cfg.Mail.PublicURL = v
	}
	if v := os.Getenv("REMOTE_BOTS_REGIONS"); v != "" {
		// Comma-separated ISO country codes; empty string clears the allowlist.
		parts := strings.Split(v, ",")
		cfg.Features.RemoteBotsRegions = nil
		for _, p := range parts {
			if p = strings.TrimSpace(strings.ToUpper(p)); p != "" {
				cfg.Features.RemoteBotsRegions = append(cfg.Features.RemoteBotsRegions, p)
			}
		}
	}
	return &cfg, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// RemoteBotsAllowed reports whether remote-bot features may run for a viewer
// country (ISO 3166-1 alpha-2). Empty allowlist = allowed everywhere.
func (f FeaturesConfig) RemoteBotsAllowed(country string) bool {
	if len(f.RemoteBotsRegions) == 0 {
		return true
	}
	c := strings.ToUpper(strings.TrimSpace(country))
	if c == "" {
		return false // deny when region known-restricted but country unknown
	}
	for _, r := range f.RemoteBotsRegions {
		if strings.EqualFold(r, c) {
			return true
		}
	}
	return false
}
