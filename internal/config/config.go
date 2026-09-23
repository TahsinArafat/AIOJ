package config

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	OAuth    OAuthConfig    `yaml:"oauth"`
	Judge    JudgeConfig    `yaml:"judge"`
	Redis    RedisConfig    `yaml:"redis"`
	AI       AIConfig       `yaml:"ai"`
	Mail     MailConfig     `yaml:"mail"`
	Features FeaturesConfig `yaml:"features"`
	LangDir  string         `yaml:"lang_dir"`
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
}

type AIConfig struct {
	Endpoint string `yaml:"endpoint"` // base URL of OpenAI-compatible API, e.g. http://localhost:8080/v1
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"` // model name served at the endpoint
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
	if v := os.Getenv("JUDGE_ENDPOINT"); v != "" {
		cfg.Judge.Endpoint = v
	}
	if v := os.Getenv("JUDGE_CONCURRENCY"); v != "" {
		cfg.Judge.Concurrency = atoi(v)
	}
	if v := os.Getenv("REDIS_URL"); v != "" {
		cfg.Redis.URL = v
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
