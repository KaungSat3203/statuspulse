package auth

import "time"

type Settings struct {
	Secure       bool
	Origins      []string
	SessionTTL   time.Duration
	RateLimitKey []byte
}

func (s Settings) CookieName() string {
	if s.Secure {
		return "__Host-statuspulse_session"
	}
	return "statuspulse_session"
}
