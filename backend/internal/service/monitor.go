package service

import (
	"context"
	"fmt"
	"net/url"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type MonitorRepository interface {
	Create(ctx context.Context, monitor *models.Monitor) error
	List(ctx context.Context) ([]models.Monitor, error)
	GetByID(ctx context.Context, id string) (*models.Monitor, error)
	Update(ctx context.Context, monitor *models.Monitor) error
	Delete(ctx context.Context, id string) error
}

type MonitorService struct {
	repo MonitorRepository
}

func NewMonitorService(repo MonitorRepository) *MonitorService {
	return &MonitorService{repo: repo}
}

func (s *MonitorService) CreateMonitor(ctx context.Context, monitor *models.Monitor) error {
	if err := validateMonitor(monitor); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, monitor); err != nil {
		return fmt.Errorf("create monitor: %w", err)
	}
	return nil
}

func (s *MonitorService) ListMonitors(ctx context.Context) ([]models.Monitor, error) {
	monitors, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list monitors: %w", err)
	}
	return monitors, nil
}

func (s *MonitorService) GetMonitor(ctx context.Context, id string) (*models.Monitor, error) {
	monitor, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get monitor: %w", err)
	}
	return monitor, nil
}

func (s *MonitorService) UpdateMonitor(
	ctx context.Context,
	id string,
	update models.MonitorUpdate,
) (*models.Monitor, error) {
	if !update.HasChanges() {
		return nil, fmt.Errorf("monitor update is required")
	}

	monitor, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get monitor: %w", err)
	}

	if update.Name != nil {
		monitor.Name = *update.Name
	}
	if update.URL != nil {
		monitor.URL = *update.URL
	}
	if update.ExpectedStatusCode != nil {
		monitor.ExpectedStatusCode = *update.ExpectedStatusCode
	}
	if update.IntervalSeconds != nil {
		monitor.IntervalSeconds = *update.IntervalSeconds
	}
	if update.TimeoutSeconds != nil {
		monitor.TimeoutSeconds = *update.TimeoutSeconds
	}
	if update.Active != nil {
		monitor.Active = *update.Active
	}

	if err := validateMonitor(monitor); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, monitor); err != nil {
		return nil, fmt.Errorf("update monitor: %w", err)
	}

	return monitor, nil
}

func (s *MonitorService) DeleteMonitor(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("monitor ID is required")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete monitor: %w", err)
	}
	return nil
}

func validateMonitor(monitor *models.Monitor) error {
	if monitor.Name == "" {
		return fmt.Errorf("monitor name is required")
	}
	if monitor.URL == "" {
		return fmt.Errorf("monitor URL is required")
	}

	parsedURL, err := url.ParseRequestURI(monitor.URL)
	if err != nil {
		return fmt.Errorf("invalid monitor URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("monitor URL must use http or https")
	}
	if monitor.ExpectedStatusCode < 100 || monitor.ExpectedStatusCode > 599 {
		return fmt.Errorf("expected status code must be between 100 and 599")
	}
	if monitor.IntervalSeconds < 5 || monitor.IntervalSeconds > 3600 {
		return fmt.Errorf("interval must be between 5 and 3600 seconds")
	}
	if monitor.TimeoutSeconds <= 0 || monitor.TimeoutSeconds > 300 {
		return fmt.Errorf("timeout must be between 1 and 300 seconds")
	}
	return nil
}
