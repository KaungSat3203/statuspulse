package service

import (
	"context"
	"fmt"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const (
	DefaultCheckHistoryLimit = 50
	MaxCheckHistoryLimit     = 100
)

type CheckHistoryMonitorRepository interface {
	GetByID(ctx context.Context, id string) (*models.Monitor, error)
}

type CheckHistoryRepository interface {
	ListByMonitorID(ctx context.Context, monitorID string, limit int) ([]models.CheckResult, error)
}

type CheckHistoryService struct {
	monitorRepo  CheckHistoryMonitorRepository
	checkLogRepo CheckHistoryRepository
}

func NewCheckHistoryService(
	monitorRepo CheckHistoryMonitorRepository,
	checkLogRepo CheckHistoryRepository,
) *CheckHistoryService {
	return &CheckHistoryService{monitorRepo: monitorRepo, checkLogRepo: checkLogRepo}
}

func (s *CheckHistoryService) ListCheckHistory(
	ctx context.Context,
	monitorID string,
	limit int,
) ([]models.CheckResult, error) {
	if !validCheckHistoryMonitorID(monitorID) {
		return nil, fmt.Errorf("invalid monitor ID")
	}
	if limit < 1 {
		return nil, fmt.Errorf("limit must be at least 1")
	}
	if limit > MaxCheckHistoryLimit {
		limit = MaxCheckHistoryLimit
	}

	if _, err := s.monitorRepo.GetByID(ctx, monitorID); err != nil {
		return nil, fmt.Errorf("get monitor: %w", err)
	}
	results, err := s.checkLogRepo.ListByMonitorID(ctx, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list check history: %w", err)
	}
	return results, nil
}

func validCheckHistoryMonitorID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, character := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !(character >= '0' && character <= '9') &&
			!(character >= 'a' && character <= 'f') &&
			!(character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}
