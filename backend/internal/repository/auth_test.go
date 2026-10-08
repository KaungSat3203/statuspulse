package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
)

func TestAuthRepository(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()
	repo := NewAuthRepository(pool)
	user, err := repo.CreateUser(ctx, "person@example.com", "encoded-hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateUser(ctx, "person@example.com", "other"); !errors.Is(err, auth.ErrDuplicateEmail) {
		t.Fatalf("duplicate=%v", err)
	}
	hash := make([]byte, 32)
	now := time.Now().UTC()
	if err := repo.CreateSession(ctx, hash, user.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	session, err := repo.FindSession(ctx, hash, now)
	if err != nil || session.User.ID != user.ID {
		t.Fatalf("session=%v,%v", session, err)
	}
	if _, err := repo.FindSession(ctx, hash, now.Add(time.Hour)); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatal("expiry boundary accepted")
	}
	if err := repo.RevokeSession(ctx, hash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindSession(ctx, hash, now); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatal("revoked session accepted")
	}
	// Separate repository instances share the same atomic budget.
	second := NewAuthRepository(pool)
	for i := 0; i < 3; i++ {
		allowed, err := second.TakeRateLimit(ctx, hash, 2, time.Minute)
		if err != nil || allowed != (i < 2) {
			t.Fatalf("attempt %d=%v,%v", i, allowed, err)
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE auth_rate_limits SET expires_at=clock_timestamp()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if allowed, err := repo.TakeRateLimit(ctx, hash, 2, time.Minute); err != nil || !allowed {
		t.Fatalf("reset=%v,%v", allowed, err)
	}
}

func TestSharedRateLimitConcurrentAttempts(t *testing.T) {
	pool := newTestDatabase(t)
	repo := NewAuthRepository(pool)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan bool, 40)
	errors := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, err := repo.TakeRateLimit(ctx, make([]byte, 32), 10, time.Minute)
			results <- allowed
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	allowed := 0
	for result := range results {
		if result {
			allowed++
		}
	}
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if allowed != 10 {
		t.Fatalf("concurrent allowed attempts = %d; want 10", allowed)
	}
}
