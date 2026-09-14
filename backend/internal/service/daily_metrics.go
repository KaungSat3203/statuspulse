package service

import (
	"context"
	"fmt"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type DailyMetricsRepository interface {
	Aggregate(ctx context.Context, monitorID string, metricDate time.Time) (*models.DailyMetric, error)
}

type DailyMetricsService struct{ repo DailyMetricsRepository }

func NewDailyMetricsService(repo DailyMetricsRepository) *DailyMetricsService {
	return &DailyMetricsService{repo: repo}
}

func (s *DailyMetricsService) AggregateMonitorDate(
	ctx context.Context,
	monitorID string,
	metricDate time.Time,
) (*models.DailyMetric, error) {
	if monitorID == "" {
		return nil, fmt.Errorf("monitor ID is required")
	}
	if metricDate.IsZero() {
		return nil, fmt.Errorf("metric date is required")
	}
	metric, err := s.repo.Aggregate(ctx, monitorID, metricDate)
	if err != nil {
		return nil, fmt.Errorf("aggregate monitor daily metrics: %w", err)
	}
	return metric, nil
}
