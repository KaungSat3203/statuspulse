package service

import (
	"context"
	"fmt"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const defaultIncidentFailureThreshold = 3

type MonitorCheckStateRepository interface {
	RecordCheckResult(
		ctx context.Context,
		result models.CheckResult,
	) (*models.Monitor, error)
}

type IncidentRepository interface {
	ListRecent(ctx context.Context, limit int) ([]models.Incident, error)
	Create(ctx context.Context, incident *models.Incident) error
	GetOpenByMonitorID(
		ctx context.Context,
		monitorID string,
	) (*models.Incident, error)
	Recover(ctx context.Context, incidentID string, recoveredAt time.Time) error
}

type IncidentService struct {
	monitorRepo      MonitorCheckStateRepository
	incidentRepo     IncidentRepository
	failureThreshold int
}

func NewIncidentService(
	monitorRepo MonitorCheckStateRepository,
	incidentRepo IncidentRepository,
) *IncidentService {
	return NewIncidentServiceWithThreshold(
		monitorRepo,
		incidentRepo,
		defaultIncidentFailureThreshold,
	)
}

func NewIncidentServiceWithThreshold(
	monitorRepo MonitorCheckStateRepository,
	incidentRepo IncidentRepository,
	failureThreshold int,
) *IncidentService {
	if failureThreshold < 1 {
		failureThreshold = defaultIncidentFailureThreshold
	}

	return &IncidentService{
		monitorRepo:      monitorRepo,
		incidentRepo:     incidentRepo,
		failureThreshold: failureThreshold,
	}
}

func (s *IncidentService) ListRecent(
	ctx context.Context,
	limit int,
) ([]models.Incident, error) {
	incidents, err := s.incidentRepo.ListRecent(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent incidents: %w", err)
	}

	return incidents, nil
}

func (s *IncidentService) ProcessCheck(
	ctx context.Context,
	result models.CheckResult,
) error {
	monitor, err := s.monitorRepo.RecordCheckResult(ctx, result)
	if err != nil {
		return fmt.Errorf("record monitor check state: %w", err)
	}

	openIncident, err := s.incidentRepo.GetOpenByMonitorID(
		ctx,
		result.MonitorID,
	)
	if err != nil {
		return fmt.Errorf("get open incident: %w", err)
	}

	if result.Reachable {
		if openIncident == nil {
			return nil
		}

		if err := s.incidentRepo.Recover(
			ctx,
			openIncident.ID,
			result.CheckedAt,
		); err != nil {
			return fmt.Errorf("recover incident: %w", err)
		}

		return nil
	}

	if monitor.ConsecutiveFailures < s.failureThreshold ||
		openIncident != nil {
		return nil
	}

	incident := &models.Incident{
		MonitorID: result.MonitorID,
		StartedAt: result.CheckedAt,
		Reason:    incidentReason(result),
	}

	if err := s.incidentRepo.Create(ctx, incident); err != nil {
		return fmt.Errorf("create incident: %w", err)
	}

	return nil
}

func incidentReason(result models.CheckResult) string {
	if result.Error != "" {
		return result.Error
	}

	return "monitor check failed"
}
