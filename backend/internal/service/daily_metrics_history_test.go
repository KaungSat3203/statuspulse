package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const metricHistoryMonitorID = "33333333-3333-3333-3333-333333333333"

type fakeDailyMetricsHistoryMonitorRepository struct {
	err    error
	called bool
}

func (r *fakeDailyMetricsHistoryMonitorRepository) GetByID(context.Context, string) (*models.Monitor, error) {
	r.called = true
	if r.err != nil {
		return nil, r.err
	}
	return &models.Monitor{ID: metricHistoryMonitorID}, nil
}

type fakeDailyMetricsHistoryRepository struct {
	metrics []models.DailyMetric
	err     error
	limit   int
	called  bool
}

func (r *fakeDailyMetricsHistoryRepository) ListByMonitorID(
	_ context.Context,
	_ string,
	limit int,
) ([]models.DailyMetric, error) {
	r.called = true
	r.limit = limit
	return r.metrics, r.err
}

func TestDailyMetricsHistoryServiceListMonitorMetrics(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		limit      int
		monitorErr error
		wantErr    bool
		wantLimit  int
		wantCalls  bool
	}{
		{name: "returns empty metrics", id: metricHistoryMonitorID, limit: 30, wantLimit: 30, wantCalls: true},
		{name: "clamps maximum limit", id: metricHistoryMonitorID, limit: 500, wantLimit: 365, wantCalls: true},
		{name: "rejects zero limit", id: metricHistoryMonitorID, limit: 0, wantErr: true},
		{name: "rejects invalid monitor ID", id: "invalid", limit: 30, wantErr: true},
		{name: "propagates missing monitor", id: metricHistoryMonitorID, limit: 30, monitorErr: pgx.ErrNoRows, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitorRepo := &fakeDailyMetricsHistoryMonitorRepository{err: tt.monitorErr}
			dailyMetricsRepo := &fakeDailyMetricsHistoryRepository{metrics: []models.DailyMetric{}}
			svc := NewDailyMetricsHistoryService(monitorRepo, dailyMetricsRepo)

			metrics, err := svc.ListMonitorMetrics(context.Background(), tt.id, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			if dailyMetricsRepo.called != tt.wantCalls {
				t.Fatalf("daily metrics repository called = %v, want %v", dailyMetricsRepo.called, tt.wantCalls)
			}
			if tt.wantCalls && dailyMetricsRepo.limit != tt.wantLimit {
				t.Fatalf("limit = %d, want %d", dailyMetricsRepo.limit, tt.wantLimit)
			}
			if tt.name == "returns empty metrics" && metrics == nil {
				t.Fatal("empty metrics should be an empty slice, not nil")
			}
			if errors.Is(err, pgx.ErrNoRows) != (tt.monitorErr == pgx.ErrNoRows) {
				t.Fatalf("missing monitor error wrapping mismatch: %v", err)
			}
		})
	}
}
