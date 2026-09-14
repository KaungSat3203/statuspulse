package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"github.com/KaungSat3203/statuspulse/backend/internal/db"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestCheckLogRepositoryListByMonitorID(t *testing.T) {
	_ = godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set")
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	monitorRepo := NewMonitorRepository(pool)
	created := make([]string, 0, 2)
	defer func() {
		for _, id := range created {
			if err := monitorRepo.Delete(ctx, id); err != nil {
				t.Errorf("clean up monitor: %v", err)
			}
		}
	}()

	createMonitor := func(name string) models.Monitor {
		t.Helper()
		monitor := models.Monitor{Name: name, URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5}
		if err := monitorRepo.Create(ctx, &monitor); err != nil {
			t.Fatalf("create monitor: %v", err)
		}
		monitor.Active = false
		if err := monitorRepo.Update(ctx, &monitor); err != nil {
			t.Fatalf("pause monitor: %v", err)
		}
		created = append(created, monitor.ID)
		return monitor
	}

	monitor := createMonitor("Check History Repository Monitor")
	otherMonitor := createMonitor("Other Check History Repository Monitor")
	base := time.Date(2026, time.January, 16, 12, 0, 0, 0, time.UTC)

	repo := NewCheckLogRepository(pool)
	if err := repo.CreateBatch(ctx, []models.CheckResult{
		{MonitorID: monitor.ID, CheckedAt: base.Add(-2 * time.Minute), StatusCode: 200, LatencyMs: 10, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: base.Add(-time.Minute), StatusCode: 500, LatencyMs: 20, Reachable: false, Error: "unexpected status"},
		{MonitorID: monitor.ID, CheckedAt: base, StatusCode: 200, LatencyMs: 30, Reachable: true},
		{MonitorID: otherMonitor.ID, CheckedAt: base.Add(time.Minute), StatusCode: 200, LatencyMs: 40, Reachable: true},
	}); err != nil {
		t.Fatalf("create check logs: %v", err)
	}

	results, err := repo.ListByMonitorID(ctx, monitor.ID, 2)
	if err != nil {
		t.Fatalf("ListByMonitorID: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if !results[0].CheckedAt.Equal(base) || !results[1].CheckedAt.Equal(base.Add(-time.Minute)) {
		t.Fatalf("results were not newest first: %+v", results)
	}
	if results[1].Error != "unexpected status" {
		t.Fatalf("error = %q, want unexpected status", results[1].Error)
	}

	empty, err := repo.ListByMonitorID(ctx, otherMonitor.ID, 50)
	if err != nil {
		t.Fatalf("list other monitor history: %v", err)
	}
	if len(empty) != 1 {
		t.Fatalf("other monitor results = %d, want 1", len(empty))
	}

	thirdMonitor := createMonitor("Empty Check History Repository Monitor")
	empty, err = repo.ListByMonitorID(ctx, thirdMonitor.ID, 50)
	if err != nil {
		t.Fatalf("list empty history: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty history = %+v, want empty slice", empty)
	}
}
