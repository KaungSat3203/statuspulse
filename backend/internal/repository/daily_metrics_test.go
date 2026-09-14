package repository

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"github.com/KaungSat3203/statuspulse/backend/internal/db"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestDailyMetricsRepositoryAggregate(t *testing.T) {
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
	monitor := &models.Monitor{Name: "Daily Metrics Test Monitor", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5}
	if err := monitorRepo.Create(ctx, monitor); err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	monitor.Active = false
	if err := monitorRepo.Update(ctx, monitor); err != nil {
		t.Fatalf("pause test monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	day := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	logs := []models.CheckResult{
		{MonitorID: monitor.ID, CheckedAt: day.Add(-time.Second), LatencyMs: 999, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: day.Add(1 * time.Hour), LatencyMs: 10, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: day.Add(2 * time.Hour), LatencyMs: 20, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: day.Add(3 * time.Hour), LatencyMs: 30, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: day.Add(4 * time.Hour), LatencyMs: 40, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: day.Add(5 * time.Hour), LatencyMs: 50, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: day.Add(24 * time.Hour), LatencyMs: 888, Reachable: true},
	}
	if err := NewCheckLogRepository(pool).CreateBatch(ctx, logs); err != nil {
		t.Fatalf("create logs: %v", err)
	}

	repo := NewDailyMetricsRepository(pool)
	metric, err := repo.Aggregate(ctx, monitor.ID, day)
	if err != nil {
		t.Fatalf("aggregate success day: %v", err)
	}
	assertMetric(t, metric, 5, 5, 30, 48)

	if _, err := repo.Aggregate(ctx, monitor.ID, day); err != nil {
		t.Fatalf("re-aggregate day: %v", err)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM daily_metrics WHERE monitor_id = $1 AND metric_date = $2`, monitor.ID, day.Format("2006-01-02")).Scan(&rows); err != nil {
		t.Fatalf("count metrics: %v", err)
	}
	if rows != 1 {
		t.Fatalf("daily metrics rows = %d, want 1", rows)
	}

	mixedDay := day.AddDate(0, 0, 2)
	mixed := []models.CheckResult{
		{MonitorID: monitor.ID, CheckedAt: mixedDay.Add(time.Hour), LatencyMs: 100, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: mixedDay.Add(2 * time.Hour), LatencyMs: 200, Reachable: false},
		{MonitorID: monitor.ID, CheckedAt: mixedDay.Add(3 * time.Hour), LatencyMs: 300, Reachable: true},
	}
	if err := NewCheckLogRepository(pool).CreateBatch(ctx, mixed); err != nil {
		t.Fatalf("create mixed logs: %v", err)
	}
	metric, err = repo.Aggregate(ctx, monitor.ID, mixedDay)
	if err != nil {
		t.Fatalf("aggregate mixed day: %v", err)
	}
	assertMetric(t, metric, 3, 2, 200, 290)

	emptyDay := day.AddDate(0, 0, 5)
	metric, err = repo.Aggregate(ctx, monitor.ID, emptyDay)
	if err != nil {
		t.Fatalf("aggregate empty day: %v", err)
	}
	if metric.TotalChecks != 0 || metric.SuccessfulChecks != 0 || metric.MeanLatencyMs != nil || metric.P95LatencyMs != nil {
		t.Fatalf("unexpected empty metric: %+v", metric)
	}
}

func assertMetric(t *testing.T, metric *models.DailyMetric, total, successful int64, mean, p95 float64) {
	t.Helper()
	if metric.TotalChecks != total || metric.SuccessfulChecks != successful {
		t.Fatalf("counts = %d/%d, want %d/%d", metric.TotalChecks, metric.SuccessfulChecks, total, successful)
	}
	if metric.MeanLatencyMs == nil || math.Abs(*metric.MeanLatencyMs-mean) > 0.001 {
		t.Fatalf("mean = %+v, want %v", metric.MeanLatencyMs, mean)
	}
	if metric.P95LatencyMs == nil || math.Abs(*metric.P95LatencyMs-p95) > 0.001 {
		t.Fatalf("p95 = %+v, want %v", metric.P95LatencyMs, p95)
	}
}
