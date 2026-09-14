package service

import (
	"context"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type fakeDailyMetricsRepository struct {
	metric    *models.DailyMetric
	monitorID string
	date      time.Time
}

func (r *fakeDailyMetricsRepository) Aggregate(_ context.Context, monitorID string, date time.Time) (*models.DailyMetric, error) {
	r.monitorID = monitorID
	r.date = date
	return r.metric, nil
}

func TestDailyMetricsServiceAggregateMonitorDate(t *testing.T) {
	repo := &fakeDailyMetricsRepository{metric: &models.DailyMetric{MonitorID: "monitor-1"}}
	service := NewDailyMetricsService(repo)
	day := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	if _, err := service.AggregateMonitorDate(context.Background(), "monitor-1", day); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if repo.monitorID != "monitor-1" || !repo.date.Equal(day) {
		t.Fatal("service did not delegate requested monitor/date")
	}
}
