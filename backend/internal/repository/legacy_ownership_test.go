package repository

import (
	"context"
	"os"
	"testing"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestExplicitLegacyOwnership(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()
	users := NewAuthRepository(pool)
	user, err := users.CreateUser(ctx, "owner@example.com", "encoded")
	if err != nil {
		t.Fatal(err)
	}
	repo := NewMonitorRepository(pool)
	monitor := &models.Monitor{Name: "legacy", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5}
	if err := repo.Create(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	finalize, err := os.ReadFile("../../operations/finalize_monitor_ownership.sql")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, string(finalize)); err == nil {
		conn.Release()
		t.Fatal("finalization accepted an unowned monitor")
	}
	if _, err := conn.Exec(ctx, "ROLLBACK"); err != nil {
		conn.Release()
		t.Fatal(err)
	}
	conn.Release()
	owned := NewUserMonitorRepository(pool)
	ownerCtx := auth.WithUser(ctx, user.ID)
	if err := repo.AssignLegacyMonitor(ctx, monitor.ID, user.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, err := owned.List(ownerCtx); err != nil || len(got) != 0 {
		t.Fatal("dry run changed ownership")
	}
	if err := repo.AssignLegacyMonitor(ctx, monitor.ID, "00000000-0000-0000-0000-000000000000", true); err == nil {
		t.Fatal("missing user accepted")
	}
	if err := repo.AssignLegacyMonitor(ctx, monitor.ID, user.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, err := owned.List(ownerCtx); err != nil || len(got) != 1 || got[0].ID != monitor.ID {
		t.Fatal("explicit assignment failed")
	}
	if err := repo.AssignLegacyMonitor(ctx, monitor.ID, user.ID, true); err == nil {
		t.Fatal("already-owned monitor reassigned")
	}
	if _, err := pool.Exec(ctx, string(finalize)); err != nil {
		t.Fatal(err)
	}
	var nullable string
	if err := pool.QueryRow(ctx, "SELECT is_nullable FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='monitors' AND column_name='user_id'").Scan(&nullable); err != nil || nullable != "NO" {
		t.Fatalf("ownership constraint = %s, %v", nullable, err)
	}
}
