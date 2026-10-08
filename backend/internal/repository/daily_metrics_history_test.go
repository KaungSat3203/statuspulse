package repository

import (
	"context"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestDailyMetricsRepositoryListByMonitorID(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()

	monitorRepo := NewMonitorRepository(pool)
	monitor := &models.Monitor{
		Name:               "Metrics History Monitor",
		URL:                "https://example.com",
		ExpectedStatusCode: 200,
		IntervalSeconds:    30,
		TimeoutSeconds:     5,
	}
	if err := monitorRepo.Create(ctx, monitor); err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	if _, err := pool.Exec(ctx, `DELETE FROM daily_metrics WHERE monitor_id = $1`, monitor.ID); err != nil {
		t.Fatalf("clean existing daily metrics: %v", err)
	}

	today := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	rows := []models.DailyMetric{
		{MonitorID: monitor.ID, MetricDate: today, TotalChecks: 10, SuccessfulChecks: 8, MeanLatencyMs: floatPtr(100), P95LatencyMs: floatPtr(200)},
		{MonitorID: monitor.ID, MetricDate: today.AddDate(0, 0, -1), TotalChecks: 5, SuccessfulChecks: 5, MeanLatencyMs: floatPtr(50), P95LatencyMs: floatPtr(80)},
	}
	for _, metric := range rows {
		if _, err := pool.Exec(ctx, `
			INSERT INTO daily_metrics (monitor_id, metric_date, total_checks, successful_checks, mean_latency_ms, p95_latency_ms)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, metric.MonitorID, metric.MetricDate, metric.TotalChecks, metric.SuccessfulChecks, metric.MeanLatencyMs, metric.P95LatencyMs); err != nil {
			t.Fatalf("insert metric row: %v", err)
		}
	}

	repo := NewDailyMetricsRepository(pool)
	metrics, err := repo.ListByMonitorID(ctx, monitor.ID, 2)
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("len(metrics) = %d, want 2", len(metrics))
	}
	if metrics[0].MetricDate != today {
		t.Fatalf("first metric date = %s, want %s", metrics[0].MetricDate.Format(time.RFC3339), today.Format(time.RFC3339))
	}
	if metrics[0].MonitorID != monitor.ID {
		t.Fatalf("metric monitor_id = %s, want %s", metrics[0].MonitorID, monitor.ID)
	}
	if metrics[0].TotalChecks != 10 || metrics[0].SuccessfulChecks != 8 {
		t.Fatalf("first metric counts = %d/%d, want 10/8", metrics[0].TotalChecks, metrics[0].SuccessfulChecks)
	}
	if metrics[0].MeanLatencyMs == nil || *metrics[0].MeanLatencyMs != 100 {
		t.Fatalf("mean latency = %v, want 100", metrics[0].MeanLatencyMs)
	}
	if metrics[0].P95LatencyMs == nil || *metrics[0].P95LatencyMs != 200 {
		t.Fatalf("p95 latency = %v, want 200", metrics[0].P95LatencyMs)
	}

	freshMonitor := &models.Monitor{Name: "No Metrics Monitor", URL: "https://example.org", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5}
	if err := monitorRepo.Create(ctx, freshMonitor); err != nil {
		t.Fatalf("create fresh monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, freshMonitor.ID)

	noMetrics, err := repo.ListByMonitorID(ctx, freshMonitor.ID, 30)
	if err != nil {
		t.Fatalf("list no metrics: %v", err)
	}
	if len(noMetrics) != 0 {
		t.Fatalf("len(noMetrics) = %d, want 0", len(noMetrics))
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
