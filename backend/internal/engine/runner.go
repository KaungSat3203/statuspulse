package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

type MonitorRepository interface {
	List(ctx context.Context) ([]models.Monitor, error)
}

type CheckLogRepository interface {
	CreateBatch(ctx context.Context, results []models.CheckResult) ([]models.CheckResult, error)
}

type IncidentService interface {
	ProcessCheck(ctx context.Context, result models.CheckResult) error
}

type TelemetryPublisher interface {
	Publish(result models.CheckResult)
}

type scheduledMonitor struct {
	monitor models.Monitor
	nextRun time.Time
}

type CheckFunc func(
	ctx context.Context,
	monitor models.Monitor,
) models.CheckResult

type Runner struct {
	monitorRepo        MonitorRepository
	checkLogRepo       CheckLogRepository
	incidentService    IncidentService
	telemetryPublisher TelemetryPublisher
	workerPool         *WorkerPool

	mu       sync.Mutex
	schedule map[string]scheduledMonitor
}

func NewRunner(
	monitorRepo MonitorRepository,
	checkLogRepo CheckLogRepository,
	incidentService IncidentService,
	telemetryPublishers ...TelemetryPublisher,
) *Runner {
	var telemetryPublisher TelemetryPublisher
	if len(telemetryPublishers) > 0 {
		telemetryPublisher = telemetryPublishers[0]
	}

	return &Runner{
		monitorRepo:        monitorRepo,
		checkLogRepo:       checkLogRepo,
		incidentService:    incidentService,
		telemetryPublisher: telemetryPublisher,
		workerPool: NewWorkerPool(3, func(
			ctx context.Context,
			monitor models.Monitor,
		) models.CheckResult {
			return CheckMonitor(ctx, monitor)
		}),
		schedule: make(map[string]scheduledMonitor),
	}
}

func (r *Runner) RunOnce(ctx context.Context) error {
	monitors, err := r.monitorRepo.List(ctx)
	if err != nil {
		return err
	}

	now := time.Now()

	var dueMonitors []models.Monitor

	r.mu.Lock()

	currentMonitorIDs := make(map[string]struct{}, len(monitors))
	for _, monitor := range monitors {
		currentMonitorIDs[monitor.ID] = struct{}{}
	}
	for monitorID := range r.schedule {
		if _, exists := currentMonitorIDs[monitorID]; !exists {
			delete(r.schedule, monitorID)
		}
	}

	for _, monitor := range monitors {
		if !monitor.Active {
			delete(r.schedule, monitor.ID)
			continue
		}

		scheduled, exists := r.schedule[monitor.ID]

		if !exists {
			scheduled = scheduledMonitor{
				monitor: monitor,
				nextRun: now,
			}
		} else {
			scheduled.monitor = monitor
		}

		if now.Before(scheduled.nextRun) {
			r.schedule[monitor.ID] = scheduled
			continue
		}

		scheduled.nextRun = now.Add(
			time.Duration(monitor.IntervalSeconds) * time.Second,
		)

		r.schedule[monitor.ID] = scheduled

		dueMonitors = append(dueMonitors, monitor)
	}

	r.mu.Unlock()

	if len(dueMonitors) == 0 {
		return nil
	}

	results := r.workerPool.Run(ctx, dueMonitors)

	if err := ctx.Err(); err != nil {
		return err
	}
	persisted, err := r.checkLogRepo.CreateBatch(ctx, results)
	if err != nil {
		// Never replay an uncertain commit automatically: without a durable
		// check identity that could duplicate logs and failure counters.
		return fmt.Errorf("save check results: %w", err)
	}

	persistedIDs := make(map[string]struct{}, len(persisted))
	for _, result := range persisted {
		persistedIDs[result.MonitorID] = struct{}{}
	}
	r.mu.Lock()
	for _, result := range results {
		if _, exists := persistedIDs[result.MonitorID]; !exists {
			delete(r.schedule, result.MonitorID)
		}
	}
	r.mu.Unlock()

	for _, result := range persisted {
		if err := r.incidentService.ProcessCheck(ctx, result); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// The monitor may have been deleted after the batch committed.
				r.mu.Lock()
				delete(r.schedule, result.MonitorID)
				r.mu.Unlock()
				continue
			}
			log.Printf(
				"failed to process incident lifecycle for monitor %s: %v",
				result.MonitorID,
				err,
			)
		}

		if r.telemetryPublisher != nil {
			r.telemetryPublisher.Publish(result)
		}
	}

	return nil
}

func (r *Runner) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("Monitoring engine started")

	for {
		if err := r.RunOnce(ctx); err != nil {
			log.Printf("monitoring engine error: %v", err)
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Println("Monitoring engine stopped")
			return
		}
	}
}
