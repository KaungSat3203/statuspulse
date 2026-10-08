package repository

import (
	"context"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestCheckLogRepository_Create(t *testing.T) {
	pool := newTestDatabase(t)

	// Create a monitor first because check_logs.monitor_id
	// references monitors.id.
	monitorRepo := NewMonitorRepository(pool)

	monitor := &models.Monitor{
		Name:               "Check Log Test Monitor",
		URL:                "https://example.com",
		ExpectedStatusCode: 200,
		IntervalSeconds:    30,
		TimeoutSeconds:     5,
	}

	err := monitorRepo.Create(context.Background(), monitor)
	if err != nil {
		t.Fatalf("create test monitor: %v", err)
	}

	// Clean up the monitor after the test.
	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM monitors WHERE id = $1",
			monitor.ID,
		)
		if err != nil {
			t.Fatalf("clean up test monitor: %v", err)
		}
	}()

	repo := NewCheckLogRepository(pool)

	result := models.CheckResult{
		MonitorID:  monitor.ID,
		CheckedAt:  time.Now(),
		StatusCode: 200,
		LatencyMs:  123,
		Reachable:  true,
	}

	_, err = repo.CreateBatch(context.Background(), []models.CheckResult{result})
	if err != nil {
		t.Fatalf("create check log: %v", err)
	}

	var (
		statusCode int
		latencyMs  int64
		reachable  bool
	)

	err = pool.QueryRow(
		context.Background(),
		`
		SELECT status_code, latency_ms, reachable
		FROM check_logs
		WHERE monitor_id = $1
		ORDER BY checked_at DESC
		LIMIT 1
		`,
		monitor.ID,
	).Scan(
		&statusCode,
		&latencyMs,
		&reachable,
	)

	if err != nil {
		t.Fatalf("read inserted check log: %v", err)
	}

	if statusCode != 200 {
		t.Errorf("expected status code %d, got %d", 200, statusCode)
	}

	if latencyMs != 123 {
		t.Errorf("expected latency %d, got %d", 123, latencyMs)
	}

	if !reachable {
		t.Error("expected reachable to be true")
	}
}

func TestCheckLogRepository_CreateBatch(t *testing.T) {
	pool := newTestDatabase(t)

	ctx := context.Background()

	monitorRepo := NewMonitorRepository(pool)

	monitor := models.Monitor{
		Name:               "Batch Test Monitor",
		URL:                "https://example.com",
		ExpectedStatusCode: 200,
		IntervalSeconds:    30,
		TimeoutSeconds:     5,
		Active:             true,
	}

	if err := monitorRepo.Create(ctx, &monitor); err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	monitor.Active = false
	if err := monitorRepo.Update(ctx, &monitor); err != nil {
		t.Fatalf("pause test monitor: %v", err)
	}

	defer monitorRepo.Delete(ctx, monitor.ID)

	results := []models.CheckResult{
		{
			MonitorID:  monitor.ID,
			CheckedAt:  time.Now(),
			StatusCode: 200,
			LatencyMs:  100,
			Reachable:  true,
		},
		{
			MonitorID:  monitor.ID,
			CheckedAt:  time.Now(),
			StatusCode: 200,
			LatencyMs:  150,
			Reachable:  true,
		},
	}

	checkLogRepo := NewCheckLogRepository(pool)

	if _, err := checkLogRepo.CreateBatch(ctx, results); err != nil {
		t.Fatalf("create batch: %v", err)
	}

	var count int

	err := pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM check_logs
		WHERE monitor_id = $1
		`,
		monitor.ID,
	).Scan(&count)

	if err != nil {
		t.Fatalf("count check logs: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 check logs, got %d", count)
	}
}
