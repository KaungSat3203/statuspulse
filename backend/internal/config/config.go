package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	Auth           auth.Settings
	TrustedProxies []string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	cfg := Config{DatabaseURL: os.Getenv("DATABASE_URL")}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set")
	}
	var err error
	cfg.Auth, err = loadAuth()
	if err != nil {
		return Config{}, err
	}
	if value := os.Getenv("TRUSTED_PROXY_CIDRS"); value != "" {
		cfg.TrustedProxies = strings.Split(value, ",")
	}
	return cfg, nil
}

func loadAuth() (auth.Settings, error) {
	env := os.Getenv("APP_ENV")
	if env != "" && env != "development" && env != "production" {
		return auth.Settings{}, fmt.Errorf("APP_ENV must be development or production")
	}
	settings := auth.Settings{Secure: env == "production", SessionTTL: 7 * 24 * time.Hour}
	origins := os.Getenv("FRONTEND_ORIGINS")
	if origins == "" && !settings.Secure {
		origins = "http://localhost:3000"
	}
	if origins == "" {
		return settings, fmt.Errorf("FRONTEND_ORIGINS is required in production")
	}
	for _, value := range strings.Split(origins, ",") {
		origin := strings.TrimSpace(value)
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" ||
			(u.Scheme != "http" && u.Scheme != "https") || (settings.Secure && u.Scheme != "https") {
			return settings, fmt.Errorf("FRONTEND_ORIGINS must contain exact HTTP origins (HTTPS in production)")
		}
		settings.Origins = append(settings.Origins, origin)
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("AUTH_RATE_LIMIT_KEY"))
	if err != nil || len(key) < 32 {
		return settings, fmt.Errorf("AUTH_RATE_LIMIT_KEY must be base64 containing at least 32 random bytes")
	}
	settings.RateLimitKey = key
	if value := os.Getenv("SESSION_TTL"); value != "" {
		ttl, err := time.ParseDuration(value)
		if err != nil || ttl < time.Hour || ttl > 30*24*time.Hour {
			return settings, fmt.Errorf("SESSION_TTL must be between 1h and 720h")
		}
		settings.SessionTTL = ttl
	}
	return settings, nil
}
