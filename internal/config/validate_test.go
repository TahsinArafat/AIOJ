package config

import (
	"strings"
	"testing"
)

func TestValidate_JWTSecret(t *testing.T) {
	const strong = "8f2c1a7d4b6e9f0a3c5d7e1b2a4c6d8e" // 32 bytes

	tests := []struct {
		name        string
		env         string
		secret      string
		wantErr     bool
		wantWarning bool
	}{
		{"strong secret in production", EnvProduction, strong, false, false},
		{"strong secret in development", EnvDevelopment, strong, false, false},
		{"empty in production", EnvProduction, "", true, false},
		{"empty in development", EnvDevelopment, "", true, false},
		{"default value in production", EnvProduction, "change-me-in-production", true, false},
		{"default value in development", EnvDevelopment, "change-me-in-production", false, true},
		{"too short in production", EnvProduction, "short-secret", true, false},
		{"too short in development", EnvDevelopment, "short-secret", false, true},
		{"repeated char in production", EnvProduction, strings.Repeat("a", 40), true, false},
		{"unset environment behaves like production", "", "change-me-in-production", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Environment: tt.env}
			cfg.Auth.JWTSecret = tt.secret

			warnings, err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil (warnings: %v)", warnings)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantWarning && len(warnings) == 0 {
				t.Fatal("expected an insecure-configuration warning")
			}
			if !tt.wantWarning && len(warnings) > 0 {
				t.Fatalf("unexpected warnings: %v", warnings)
			}
		})
	}
}

func TestValidate_ProductionCatcherWarns(t *testing.T) {
	cfg := &Config{Environment: EnvProduction}
	cfg.Auth.JWTSecret = "8f2c1a7d4b6e9f0a3c5d7e1b2a4c6d8e"
	cfg.Mail.Driver = "catcher"

	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning about mail.driver=catcher in production")
	}
}

func TestIsDevelopment(t *testing.T) {
	for _, env := range []string{"development", "Development", " DEVELOPMENT "} {
		if !(&Config{Environment: env}).IsDevelopment() {
			t.Errorf("%q should be development", env)
		}
	}
	for _, env := range []string{"", "production", "dev", "staging"} {
		if (&Config{Environment: env}).IsDevelopment() {
			t.Errorf("%q must not be treated as development", env)
		}
	}
}

func TestRedisAddrAndPassword(t *testing.T) {
	tests := []struct {
		name     string
		cfg      RedisConfig
		wantAddr string
		wantPass string
	}{
		{"bare host:port", RedisConfig{URL: "redis:6379"}, "redis:6379", ""},
		{"empty", RedisConfig{}, "", ""},
		{"url form", RedisConfig{URL: "redis://cache:6380"}, "cache:6380", ""},
		{"url with password", RedisConfig{URL: "redis://:s3cret@cache:6380"}, "cache:6380", "s3cret"},
		{"explicit password wins", RedisConfig{URL: "redis://:url-pass@cache:6380", Password: "env-pass"}, "cache:6380", "env-pass"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Addr(); got != tt.wantAddr {
				t.Errorf("Addr() = %q, want %q", got, tt.wantAddr)
			}
			if got := tt.cfg.ResolvePassword(); got != tt.wantPass {
				t.Errorf("ResolvePassword() = %q, want %q", got, tt.wantPass)
			}
		})
	}
}
