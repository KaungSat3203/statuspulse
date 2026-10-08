package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidInput       = errors.New("invalid authentication input")
	ErrDuplicateEmail     = errors.New("unable to create account with this email")
	ErrUnauthenticated    = errors.New("authentication required")
	ErrBusy               = errors.New("authentication busy; try again")
)

// Limits concurrent Argon2 memory use per process (2 * 64 MiB).
type PasswordHasher struct {
	slots chan struct{}
}

func NewPasswordHasher() *PasswordHasher { return &PasswordHasher{slots: make(chan struct{}, 2)} }

func NormalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 || !strings.Contains(value, "@") {
		return "", ErrInvalidInput
	}
	return value, nil
}

func ValidPassword(password string) bool {
	n := utf8.RuneCountInString(password)
	return utf8.ValidString(password) && n >= 15 && n <= 128 && len(password) <= 512
}

func (h *PasswordHasher) derive(ctx context.Context, password string, salt []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return nil, ErrBusy
	}
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32), nil
}

func (h *PasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	if !ValidPassword(password) {
		return "", ErrInvalidInput
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash, err := h.derive(ctx, password, salt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func (h *PasswordHasher) Verify(ctx context.Context, encoded, password string) (bool, error) {
	// Accept only the bounded parameters this application writes. Never let a
	// corrupt/untrusted encoded hash request arbitrary memory or CPU.
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=1" {
		return false, errors.New("invalid stored password hash")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false, errors.New("invalid stored password salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false, errors.New("invalid stored password digest")
	}
	actual, err := h.derive(ctx, password, salt)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}
