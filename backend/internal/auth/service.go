package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type Store interface {
	CreateUser(context.Context, string, string) (*models.User, error)
	FindUser(context.Context, string) (*models.User, error)
	CreateSession(context.Context, []byte, string, time.Time) error
	FindSession(context.Context, []byte, time.Time) (*models.Session, error)
	RevokeSession(context.Context, []byte, time.Time) error
	TakeRateLimit(context.Context, []byte, int, time.Duration) (bool, error)
}

type Service struct {
	store     Store
	passwords *PasswordHasher
	dummyHash string
	ttl       time.Duration
	rateKey   []byte
	now       func() time.Time
}

func NewService(store Store, ttl time.Duration, rateKey []byte) (*Service, error) {
	if ttl <= 0 || len(rateKey) < 32 {
		return nil, errors.New("invalid auth configuration")
	}
	passwords := NewPasswordHasher()
	dummy, err := passwords.Hash(context.Background(), "dummy verification password")
	if err != nil {
		return nil, err
	}
	return &Service{store: store, passwords: passwords, dummyHash: dummy, ttl: ttl, rateKey: rateKey, now: time.Now}, nil
}

func (s *Service) Signup(ctx context.Context, email, password string) (*models.User, error) {
	email, err := NormalizeEmail(email)
	if err != nil || !ValidPassword(password) {
		return nil, ErrInvalidInput
	}
	hash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return nil, err
	}
	return s.store.CreateUser(ctx, email, hash)
}

func (s *Service) Login(ctx context.Context, email, password string) (*models.User, error) {
	normalized, emailErr := NormalizeEmail(email)
	if len(password) > 512 {
		return nil, ErrInvalidCredentials
	}
	var user *models.User
	if emailErr == nil {
		var err error
		user, err = s.store.FindUser(ctx, normalized)
		if err != nil && !errors.Is(err, ErrUnauthenticated) {
			return nil, err
		}
	}
	hash := s.dummyHash
	if user != nil {
		hash = user.PasswordHash
	}
	valid, err := s.passwords.Verify(ctx, hash, password)
	if err != nil {
		return nil, err
	}
	if !valid || user == nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

func tokenHash(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return nil, ErrUnauthenticated
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}

func (s *Service) CreateSession(ctx context.Context, userID string) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	expires := s.now().UTC().Add(s.ttl)
	if err := s.store.CreateSession(ctx, hash[:], userID, expires); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (*models.Session, error) {
	hash, err := tokenHash(token)
	if err != nil {
		return nil, err
	}
	session, err := s.store.FindSession(ctx, hash, s.now().UTC())
	if err != nil {
		return nil, err
	}
	if session == nil || !session.ExpiresAt.After(s.now().UTC()) {
		return nil, ErrUnauthenticated
	}
	return session, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	hash, err := tokenHash(token)
	if err != nil {
		return nil
	} // Idempotent for malformed/missing cookies.
	return s.store.RevokeSession(ctx, hash, s.now().UTC())
}

// Each request consumes both a shared IP bucket and an account bucket. The
// database uses its own clock so API-instance clock skew cannot reset limits.
func (s *Service) AllowAttempt(ctx context.Context, action, ip, email string) (bool, error) {
	normalized, _ := NormalizeEmail(email)
	keys := []struct {
		value string
		limit int
	}{
		{action + ":ip:" + ip, 30},
		{action + ":account:" + normalized, 10},
	}
	for _, key := range keys {
		mac := hmac.New(sha256.New, s.rateKey)
		_, _ = mac.Write([]byte(key.value))
		allowed, err := s.store.TakeRateLimit(ctx, mac.Sum(nil), key.limit, 15*time.Minute)
		if err != nil || !allowed {
			return false, err
		}
	}
	return true, nil
}

type userKey struct{}

// Only authentication middleware supplies this value; JSON/headers cannot.
func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userKey{}, userID)
}

func UserID(ctx context.Context) (string, error) {
	id, _ := ctx.Value(userKey{}).(string)
	if id == "" {
		return "", ErrUnauthenticated
	}
	return id, nil
}
