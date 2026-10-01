package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration, sourced from environment variables.
// A .env file in the working directory is loaded first when present.
type Config struct {
	Env       string
	Addr      string
	AppURL    string
	LogFormat string

	DatabaseURL string

	SessionTTL          time.Duration
	SessionCookieSecure bool

	// Timezone groups reports and month boundaries for the user.
	Timezone string

	BelvoEnv      string
	BelvoSecretID string
	BelvoSecret   string

	AIBaseURL string
	AIAPIKey  string
	AIModel   string
}

func (c *Config) IsDev() bool { return c.Env != "prod" }

func (c *Config) SessionCookieName() string { return "okane_session" }

// Location resolves the configured timezone.
func (c *Config) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("OKANE_TIMEZONE %q is not a known IANA timezone: %w", c.Timezone, err)
	}
	return loc, nil
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{
		Env:       env("OKANE_ENV", "dev"),
		Addr:      env("OKANE_ADDR", ":8080"),
		AppURL:    strings.TrimRight(env("OKANE_APP_URL", "http://localhost:5173"), "/"),
		LogFormat: env("OKANE_LOG_FORMAT", "text"),

		DatabaseURL: env("OKANE_DATABASE_URL", ""),

		SessionTTL:          envDuration("OKANE_SESSION_TTL", 30*24*time.Hour),
		SessionCookieSecure: envBool("OKANE_SESSION_COOKIE_SECURE", false),

		Timezone: env("OKANE_TIMEZONE", "America/Mexico_City"),

		BelvoEnv:      env("OKANE_BELVO_ENV", "sandbox"),
		BelvoSecretID: env("OKANE_BELVO_SECRET_ID", ""),
		BelvoSecret:   env("OKANE_BELVO_SECRET_PASSWORD", ""),

		AIBaseURL: strings.TrimRight(env("OKANE_AI_BASE_URL", "https://opencode.ai/inference/openai/v1"), "/"),
		AIAPIKey:  env("OKANE_AI_API_KEY", ""),
		AIModel:   env("OKANE_AI_MODEL", "glm-5.1"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("OKANE_DATABASE_URL is required")
	}
	if cfg.IsDev() {
		// Development without TLS: cookies must not be marked Secure.
		cfg.SessionCookieSecure = envBool("OKANE_SESSION_COOKIE_SECURE", false)
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
