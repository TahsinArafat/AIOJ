package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Judge    JudgeConfig    `yaml:"judge"`
	Redis    RedisConfig    `yaml:"redis"`
	AI       AIConfig       `yaml:"ai"`
	LangDir  string         `yaml:"lang_dir"`
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
	JWTSecret  string `yaml:"jwt_secret"`
	AccessTTL  string `yaml:"access_ttl"`
	RefreshTTL string `yaml:"refresh_ttl"`
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
	return &cfg, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
