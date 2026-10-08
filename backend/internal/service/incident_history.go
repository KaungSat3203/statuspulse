package service

import (
	"context"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

// The API only needs incident reads; it never receives lifecycle write methods.
type IncidentHistoryRepository interface {
	ListRecent(context.Context, int) ([]models.Incident, error)
}
type IncidentHistoryService struct{ repo IncidentHistoryRepository }

func NewIncidentHistoryService(repo IncidentHistoryRepository) *IncidentHistoryService {
	return &IncidentHistoryService{repo}
}
func (s *IncidentHistoryService) ListRecent(ctx context.Context, limit int) ([]models.Incident, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListRecent(ctx, limit)
}
