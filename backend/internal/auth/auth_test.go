package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type memoryStore struct {
	user    *models.User
	hash    []byte
	expires time.Time
	revoked bool
}

func (m *memoryStore) CreateUser(_ context.Context, email, hash string) (*models.User, error) {
	if m.user != nil {
		return nil, ErrDuplicateEmail
	}
	m.user = &models.User{ID: "user-id", Email: email, PasswordHash: hash}
	return m.user, nil
}
func (m *memoryStore) FindUser(_ context.Context, email string) (*models.User, error) {
	if m.user == nil || m.user.Email != email {
		return nil, ErrUnauthenticated
	}
	return m.user, nil
}
func (m *memoryStore) CreateSession(_ context.Context, hash []byte, _ string, expires time.Time) error {
	m.hash = append([]byte(nil), hash...)
	m.expires = expires
	m.revoked = false
	return nil
}
func (m *memoryStore) FindSession(_ context.Context, hash []byte, now time.Time) (*models.Session, error) {
	if m.revoked || string(hash) != string(m.hash) || !m.expires.After(now) {
		return nil, ErrUnauthenticated
	}
	return &models.Session{User: *m.user, ExpiresAt: m.expires}, nil
}
func (m *memoryStore) RevokeSession(_ context.Context, _ []byte, _ time.Time) error {
	m.revoked = true
	return nil
}
func (m *memoryStore) TakeRateLimit(context.Context, []byte, int, time.Duration) (bool, error) {
	return true, nil
}

func TestPasswordsAndEmail(t *testing.T) {
	ctx := context.Background()
	h := NewPasswordHasher()
	password := "a long password with spaces"
	first, err := h.Hash(ctx, password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.Hash(ctx, password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || strings.Contains(first, password) {
		t.Fatal("hash must use a unique salt and not contain password")
	}
	for _, tc := range []struct {
		password string
		want     bool
	}{{password, true}, {"wrong password", false}} {
		got, err := h.Verify(ctx, first, tc.password)
		if err != nil || got != tc.want {
			t.Fatalf("verify=%v,%v", got, err)
		}
	}
	if _, err := h.Verify(ctx, strings.Replace(first, "m=65536", "m=999999999", 1), password); err == nil {
		t.Fatal("unbounded parameters accepted")
	}
	for _, password := range []string{"short", strings.Repeat("a", 129)} {
		if ValidPassword(password) {
			t.Fatal("invalid length accepted")
		}
	}
	email, err := NormalizeEmail("  Person@Example.COM ")
	if err != nil || email != "person@example.com" {
		t.Fatalf("normalize=%q,%v", email, err)
	}
	for _, email := range []string{"not-email", "Name <person@example.com>", "a\nb@example.com"} {
		if _, err := NormalizeEmail(email); err == nil {
			t.Fatal("invalid email accepted")
		}
	}
}

func TestSignupLoginAndSessions(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{}
	s, err := NewService(store, 7*24*time.Hour, []byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	if _, err := s.Signup(ctx, "bad", "short"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	user, err := s.Signup(ctx, " PERSON@example.com ", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "person@example.com" {
		t.Fatal("email not normalized")
	}
	if _, err := s.Signup(ctx, user.Email, "correct horse battery staple"); !errors.Is(err, ErrDuplicateEmail) {
		t.Fatal(err)
	}
	if _, err := s.Login(ctx, "PERSON@example.com", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{user.Email, "missing@example.com", "invalid"} {
		if _, err := s.Login(ctx, email, "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("login=%v", err)
		}
	}
	token, expires, err := s.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(token)
	expected := sha256.Sum256(raw)
	if string(store.hash) != string(expected[:]) || string(store.hash) == token {
		t.Fatal("session must store only hash")
	}
	if expires != now.Add(7*24*time.Hour) {
		t.Fatal("wrong expiry")
	}
	if _, err := s.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, "malformed"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	now = expires
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("expired session accepted")
	}
	token, _, err = s.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("revoked session accepted")
	}
}
