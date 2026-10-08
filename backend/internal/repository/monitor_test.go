package repository

import (
	"context"
	"testing"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestMonitorRepository_Create(t *testing.T) {
	pool := newTestDatabase(t)

	repo := NewMonitorRepository(pool)

	monitor := &models.Monitor{
		Name:               "Test Monitor",
		URL:                "https://example.com",
		ExpectedStatusCode: 200,
		IntervalSeconds:    30,
		TimeoutSeconds:     5,
	}

	err := repo.Create(context.Background(), monitor)
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	if monitor.ID == "" {
		t.Fatal("expected monitor ID to be populated")
	}

	if monitor.Name != "Test Monitor" {
		t.Errorf("expected name %q, got %q", "Test Monitor", monitor.Name)
	}

	if monitor.URL != "https://example.com" {
		t.Errorf("expected URL %q, got %q", "https://example.com", monitor.URL)
	}

	if monitor.ExpectedStatusCode != 200 {
		t.Errorf("expected status %d, got %d", 200, monitor.ExpectedStatusCode)
	}

	if monitor.IntervalSeconds != 30 {
		t.Errorf("expected interval %d, got %d", 30, monitor.IntervalSeconds)
	}

	if monitor.TimeoutSeconds != 5 {
		t.Errorf("expected timeout %d, got %d", 5, monitor.TimeoutSeconds)
	}

	monitor.Active = false
	err = repo.Update(context.Background(), monitor)
	if err != nil {
		t.Fatalf("pause monitor: %v", err)
	}
	if monitor.Active {
		t.Fatal("expected monitor to be inactive")
	}

	// Remove the test row so repeated test runs stay clean.
	_, err = pool.Exec(
		context.Background(),
		"DELETE FROM monitors WHERE id = $1",
		monitor.ID,
	)
	if err != nil {
		t.Fatalf("clean up test monitor: %v", err)
	}
}
