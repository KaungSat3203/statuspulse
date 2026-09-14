package engine

import (
	"context"
	"testing"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type recordingTelemetryPublisher struct {
	results []models.CheckResult
}

func (p *recordingTelemetryPublisher) Publish(result models.CheckResult) {
	p.results = append(p.results, result)
}

func TestRunnerPublishesCheckResults(t *testing.T) {
	monitorRepo := &fakeMonitorRepository{
		monitors: []models.Monitor{
			{
				ID:                 "active-monitor",
				URL:                "https://example.com",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
				Active:             true,
			},
		},
	}
	checkLogRepo := &fakeCheckLogRepository{}
	incidentService := &fakeIncidentService{}
	telemetryPublisher := &recordingTelemetryPublisher{}

	expected := models.CheckResult{
		MonitorID:  "active-monitor",
		StatusCode: 200,
		Reachable:  true,
	}

	runner := NewRunner(
		monitorRepo,
		checkLogRepo,
		incidentService,
		telemetryPublisher,
	)
	runner.workerPool = NewWorkerPool(1, func(
		ctx context.Context,
		monitor models.Monitor,
	) models.CheckResult {
		return expected
	})

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if len(telemetryPublisher.results) != 1 {
		t.Fatalf(
			"expected one published result, got %d",
			len(telemetryPublisher.results),
		)
	}
	if telemetryPublisher.results[0] != expected {
		t.Fatalf(
			"expected published result %+v, got %+v",
			expected,
			telemetryPublisher.results[0],
		)
	}
}
