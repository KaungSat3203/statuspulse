package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAuthSettings(t *testing.T) {
	t.Setenv("AUTH_RATE_LIMIT_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	t.Setenv("SESSION_TTL", "168h")
	t.Setenv("APP_ENV", "development")
	t.Setenv("FRONTEND_ORIGINS", "")
	settings, err := loadAuth()
	if err != nil || settings.Secure || settings.Origins[0] != "http://localhost:3000" {
		t.Fatalf("development=%v", err)
	}
	t.Setenv("APP_ENV", "production")
	if _, err := loadAuth(); err == nil {
		t.Fatal("missing production origin accepted")
	}
	t.Setenv("FRONTEND_ORIGINS", "http://example.com")
	if _, err := loadAuth(); err == nil {
		t.Fatal("HTTP production origin accepted")
	}
	t.Setenv("FRONTEND_ORIGINS", "https://app.example.com")
	settings, err = loadAuth()
	if err != nil || !settings.Secure || settings.CookieName() != "__Host-statuspulse_session" {
		t.Fatalf("production=%v", err)
	}
	t.Setenv("AUTH_RATE_LIMIT_KEY", "")
	if _, err := loadAuth(); err == nil {
		t.Fatal("missing shared secret accepted")
	}
}
