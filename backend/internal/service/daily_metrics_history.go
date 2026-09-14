package service

import (
	"context"
	"fmt"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const (
	DefaultDailyMetricsLimit = 30
	MaxDailyMetricsLimit     = 365
)

type DailyMetricsHistoryMonitorRepository interface {
	GetByID(ctx context.Context, id string) (*models.Monitor, error)
}

type DailyMetricsHistoryRepository interface {
	ListByMonitorID(ctx context.Context, monitorID string, limit int) ([]models.DailyMetric, error)
}

type DailyMetricsHistoryService struct {
	monitorRepo  DailyMetricsHistoryMonitorRepository
	metricsRepo  DailyMetricsHistoryRepository
}

func NewDailyMetricsHistoryService(
	monitorRepo DailyMetricsHistoryMonitorRepository,
	metricsRepo DailyMetricsHistoryRepository,
) *DailyMetricsHistoryService {
	return &DailyMetricsHistoryService{monitorRepo: monitorRepo, metricsRepo: metricsRepo}
}

func (s *DailyMetricsHistoryService) ListMonitorMetrics(
	ctx context.Context,
	monitorID string,
	limit int,
) ([]models.DailyMetric, error) {
	if !validCheckHistoryMonitorID(monitorID) {
		return nil, fmt.Errorf("invalid monitor ID")
	}
	if limit < 1 {
		return nil, fmt.Errorf("limit must be at least 1")
	}
	if limit > MaxDailyMetricsLimit {
		limit = MaxDailyMetricsLimit
	}

	if _, err := s.monitorRepo.GetByID(ctx, monitorID); err != nil {
		return nil, fmt.Errorf("get monitor: %w", err)
	}

	metrics, err := s.metricsRepo.ListByMonitorID(ctx, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list daily metrics: %w", err)
	}
	if metrics == nil {
		return []models.DailyMetric{}, nil
	}
	return metrics, nil
}
