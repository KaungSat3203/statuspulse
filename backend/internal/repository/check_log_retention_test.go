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

func TestCheckLogRepositoryDeleteOlderThan(t *testing.T) {
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
	var createdMonitorIDs []string
	defer func() {
		for _, id := range createdMonitorIDs {
			if err := monitorRepo.Delete(ctx, id); err != nil {
				t.Errorf("clean up monitor: %v", err)
			}
		}
	}()

	createMonitor := func(name string) models.Monitor {
		t.Helper()
		monitor := models.Monitor{
			Name:               name,
			URL:                "https://example.com",
			ExpectedStatusCode: 200,
			IntervalSeconds:    30,
			TimeoutSeconds:     5,
		}
		if err := monitorRepo.Create(ctx, &monitor); err != nil {
			t.Fatalf("create monitor: %v", err)
		}
		monitor.Active = false
		if err := monitorRepo.Update(ctx, &monitor); err != nil {
			t.Fatalf("pause monitor: %v", err)
		}
		createdMonitorIDs = append(createdMonitorIDs, monitor.ID)
		return monitor
	}

	first := createMonitor("Check Log Retention First")
	second := createMonitor("Check Log Retention Second")
	cutoff := time.Date(2026, time.January, 16, 0, 0, 0, 0, time.UTC)

	checkLogRepo := NewCheckLogRepository(pool)
	if err := checkLogRepo.CreateBatch(ctx, []models.CheckResult{
		{MonitorID: first.ID, CheckedAt: cutoff.Add(time.Second), StatusCode: 200, LatencyMs: 10, Reachable: true},
		{MonitorID: first.ID, CheckedAt: cutoff, StatusCode: 200, LatencyMs: 20, Reachable: true},
		{MonitorID: first.ID, CheckedAt: cutoff.Add(-time.Second), StatusCode: 500, LatencyMs: 30, Reachable: false},
		{MonitorID: second.ID, CheckedAt: cutoff.Add(-time.Hour), StatusCode: 500, LatencyMs: 40, Reachable: false},
	}); err != nil {
		t.Fatalf("create logs: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO incidents (monitor_id, started_at, reason) VALUES ($1, $2, $3)`,
		first.ID, cutoff.Add(-time.Hour), "retention test",
	); err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO daily_metrics (monitor_id, metric_date, total_checks, successful_checks) VALUES ($1, $2, $3, $4)`,
		first.ID, cutoff.AddDate(0, 0, -1), 1, 1,
	); err != nil {
		t.Fatalf("create daily metric: %v", err)
	}

	deleted, err := checkLogRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	var remaining int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM check_logs WHERE monitor_id = $1`, first.ID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count remaining logs: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("remaining first-monitor logs = %d, want 2", remaining)
	}

	var secondRemaining int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM check_logs WHERE monitor_id = $1`, second.ID,
	).Scan(&secondRemaining); err != nil {
		t.Fatalf("count second-monitor logs: %v", err)
	}
	if secondRemaining != 0 {
		t.Fatalf("remaining second-monitor logs = %d, want 0", secondRemaining)
	}

	var incidentCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM incidents WHERE monitor_id = $1`, first.ID,
	).Scan(&incidentCount); err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	if incidentCount != 1 {
		t.Fatalf("incidents = %d, want 1", incidentCount)
	}

	var metricCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM daily_metrics WHERE monitor_id = $1`, first.ID,
	).Scan(&metricCount); err != nil {
		t.Fatalf("count daily metrics: %v", err)
	}
	if metricCount != 1 {
		t.Fatalf("daily metrics = %d, want 1", metricCount)
	}
}
