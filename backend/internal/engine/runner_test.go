package engine

import (
	"context"
	"testing"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type fakeMonitorRepository struct {
	monitors []models.Monitor
}

func (f *fakeMonitorRepository) List(ctx context.Context) ([]models.Monitor, error) {
	return f.monitors, nil
}

type fakeCheckLogRepository struct {
	results []models.CheckResult
	err     error
	persist func([]models.CheckResult) []models.CheckResult
	calls   int
}

func (f *fakeCheckLogRepository) CreateBatch(
	ctx context.Context,
	results []models.CheckResult,
) ([]models.CheckResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.persist != nil {
		results = f.persist(results)
	}
	f.results = append(f.results, results...)
	return results, nil
}

type fakeIncidentService struct {
	results []models.CheckResult
}

func (f *fakeIncidentService) ProcessCheck(
	ctx context.Context,
	result models.CheckResult,
) error {
	f.results = append(f.results, result)
	return nil
}

func TestRunner_RunOnce(t *testing.T) {
	activeServerURL := "https://example.com"
	inactiveServerURL := "https://example.org"

	monitorRepo := &fakeMonitorRepository{
		monitors: []models.Monitor{
			{
				ID:                 "active-monitor",
				Name:               "Active Monitor",
				URL:                activeServerURL,
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
				Active:             true,
			},
			{
				ID:                 "inactive-monitor",
				Name:               "Inactive Monitor",
				URL:                inactiveServerURL,
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
				Active:             false,
			},
		},
	}

	checkLogRepo := &fakeCheckLogRepository{}
	incidentService := &fakeIncidentService{}

	fakeCheck := func(
		ctx context.Context,
		monitor models.Monitor,
	) models.CheckResult {
		return models.CheckResult{
			MonitorID:  monitor.ID,
			StatusCode: 200,
			LatencyMs:  50,
			Reachable:  true,
		}
	}

	runner := NewRunner(monitorRepo, checkLogRepo, incidentService)
	runner.workerPool = NewWorkerPool(3, fakeCheck)
	err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if len(checkLogRepo.results) != 1 {
		t.Fatalf(
			"expected 1 check result, got %d",
			len(checkLogRepo.results),
		)
	}
	if len(incidentService.results) != 1 {
		t.Fatalf(
			"expected 1 incident service result, got %d",
			len(incidentService.results),
		)
	}

	result := checkLogRepo.results[0]

	if result.MonitorID != "active-monitor" {
		t.Errorf(
			"expected monitor ID %q, got %q",
			"active-monitor",
			result.MonitorID,
		)
	}

	if result.StatusCode != 200 {
		t.Errorf(
			"expected status code 200, got %d",
			result.StatusCode,
		)
	}

	if !result.Reachable {
		t.Error("expected active monitor to be reachable")
	}

	if incidentService.results[0] != result {
		t.Error("expected the incident service to receive the probe result")
	}
}
