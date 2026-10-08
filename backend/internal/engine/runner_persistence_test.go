package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

func persistenceTestRunner() (*Runner, *fakeMonitorRepository, *fakeCheckLogRepository, *fakeIncidentService, *recordingTelemetryPublisher) {
	monitors := &fakeMonitorRepository{monitors: []models.Monitor{
		{ID: "deleted", Active: true},
		{ID: "survivor", Active: true},
	}}
	logs := &fakeCheckLogRepository{}
	incidents := &fakeIncidentService{}
	publisher := &recordingTelemetryPublisher{}
	runner := NewRunner(monitors, logs, incidents, publisher)
	runner.workerPool = NewWorkerPool(1, func(_ context.Context, monitor models.Monitor) models.CheckResult {
		return models.CheckResult{
			MonitorID: monitor.ID, CheckedAt: time.Now().UTC(), StatusCode: 500, Reachable: false,
		}
	})
	return runner, monitors, logs, incidents, publisher
}

func TestRunnerPersistenceFailureDoesNotAdvanceIncidentsOrSSE(t *testing.T) {
	runner, _, logs, incidents, publisher := persistenceTestRunner()
	failure := errors.New("database unavailable")
	logs.err = failure
	if err := runner.RunOnce(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("RunOnce error = %v, want persistence failure", err)
	}
	if len(incidents.results) != 0 || len(publisher.results) != 0 {
		t.Fatal("uncommitted results reached incidents or SSE")
	}
	if logs.calls != 1 {
		t.Fatalf("write attempts = %d, want 1 (no unsafe automatic replay)", logs.calls)
	}

	logs.err = nil
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(incidents.results) != 2 || len(publisher.results) != 2 || len(logs.results) != 2 {
		t.Fatal("the next successful cycle should process each new result exactly once")
	}
}

func TestRunnerOnlyProcessesCommittedSurvivors(t *testing.T) {
	runner, monitors, logs, incidents, publisher := persistenceTestRunner()
	for i := range monitors.monitors {
		monitors.monitors[i].IntervalSeconds = 3600
	}
	logs.persist = func(results []models.CheckResult) []models.CheckResult {
		var committed []models.CheckResult
		for _, result := range results {
			if result.MonitorID == "survivor" {
				committed = append(committed, result)
			}
		}
		return committed
	}
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(incidents.results) != 1 || incidents.results[0].MonitorID != "survivor" {
		t.Fatalf("incident results = %+v", incidents.results)
	}
	if len(publisher.results) != 1 || publisher.results[0].MonitorID != "survivor" {
		t.Fatalf("SSE results = %+v", publisher.results)
	}
	if _, exists := runner.schedule["deleted"]; exists {
		t.Fatal("deleted monitor still has a schedule")
	}
	monitors.monitors = monitors.monitors[1:]
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logs.calls != 1 {
		t.Fatal("a deleted monitor or not-yet-due monitor was checked again")
	}
}

type persistenceIncidentFunc func(context.Context, models.CheckResult) error

func (f persistenceIncidentFunc) ProcessCheck(ctx context.Context, result models.CheckResult) error {
	return f(ctx, result)
}

func TestRunnerSkipsMonitorDeletedAfterCommit(t *testing.T) {
	runner, _, _, _, publisher := persistenceTestRunner()
	runner.incidentService = persistenceIncidentFunc(func(_ context.Context, result models.CheckResult) error {
		if result.MonitorID == "deleted" {
			return fmt.Errorf("record check state: %w", pgx.ErrNoRows)
		}
		return nil
	})
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(publisher.results) != 1 || publisher.results[0].MonitorID != "survivor" {
		t.Fatalf("post-delete SSE results = %+v", publisher.results)
	}
	if _, exists := runner.schedule["deleted"]; exists {
		t.Fatal("post-commit deletion left an active schedule")
	}
}

func TestRunnerCancellationDoesNotPersistCancelledProbes(t *testing.T) {
	runner, _, logs, incidents, publisher := persistenceTestRunner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.workerPool = NewWorkerPool(1, func(_ context.Context, monitor models.Monitor) models.CheckResult {
		cancel()
		return models.CheckResult{MonitorID: monitor.ID, Error: "context canceled"}
	})
	if err := runner.RunOnce(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("RunOnce error = %v, want cancellation", err)
	}
	if logs.calls != 0 || len(incidents.results) != 0 || len(publisher.results) != 0 {
		t.Fatal("shutdown cancellation was treated as a persisted failed check")
	}
}

func TestRunnerIncidentFailureDoesNotStopOtherCommittedResults(t *testing.T) {
	runner, _, logs, _, publisher := persistenceTestRunner()
	var processed []string
	runner.incidentService = persistenceIncidentFunc(func(_ context.Context, result models.CheckResult) error {
		processed = append(processed, result.MonitorID)
		if result.MonitorID == "deleted" {
			return errors.New("incident repository unavailable")
		}
		return nil
	})
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(logs.results) != 2 || len(processed) != 2 || len(publisher.results) != 2 {
		t.Fatal("one incident error stopped delivery of other committed checks")
	}
}
