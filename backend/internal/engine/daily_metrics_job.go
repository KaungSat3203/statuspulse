package engine

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type DailyMetricsMonitorRepository interface {
	List(ctx context.Context) ([]models.Monitor, error)
}

type DailyMetricsAggregator interface {
	AggregateMonitorDate(ctx context.Context, monitorID string, metricDate time.Time) (*models.DailyMetric, error)
}

type dailyMetricsTimer interface {
	Chan() <-chan time.Time
	Stop() bool
}

type standardDailyMetricsTimer struct{ timer *time.Timer }

func (t standardDailyMetricsTimer) Chan() <-chan time.Time { return t.timer.C }
func (t standardDailyMetricsTimer) Stop() bool             { return t.timer.Stop() }

type DailyMetricsJob struct {
	monitorRepo DailyMetricsMonitorRepository
	aggregator  DailyMetricsAggregator
	now         func() time.Time
	newTimer    func(time.Duration) dailyMetricsTimer
	logf        func(string, ...any)
}

func NewDailyMetricsJob(
	monitorRepo DailyMetricsMonitorRepository,
	aggregator DailyMetricsAggregator,
) *DailyMetricsJob {
	return NewDailyMetricsJobWithClock(monitorRepo, aggregator, time.Now, func(d time.Duration) dailyMetricsTimer {
		return standardDailyMetricsTimer{timer: time.NewTimer(d)}
	})
}

func NewDailyMetricsJobWithClock(
	monitorRepo DailyMetricsMonitorRepository,
	aggregator DailyMetricsAggregator,
	now func() time.Time,
	newTimer func(time.Duration) dailyMetricsTimer,
) *DailyMetricsJob {
	return &DailyMetricsJob{monitorRepo: monitorRepo, aggregator: aggregator, now: now, newTimer: newTimer, logf: log.Printf}
}

func (j *DailyMetricsJob) Run(ctx context.Context) {
	for {
		timer := j.newTimer(untilNextUTCDay(j.now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.Chan():
		}

		if err := j.RunOnce(ctx); err != nil && ctx.Err() == nil {
			j.logf("daily metrics aggregation failed: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (j *DailyMetricsJob) RunOnce(ctx context.Context) error {
	metricDate := previousUTCDay(j.now())
	monitors, err := j.monitorRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list monitors for %s: %w", metricDate.Format("2006-01-02"), err)
	}

	for _, monitor := range monitors {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := j.aggregator.AggregateMonitorDate(ctx, monitor.ID, metricDate); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			j.logf("aggregate daily metrics for monitor %s on %s: %v", monitor.ID, metricDate.Format("2006-01-02"), err)
		}
	}
	return nil
}

func previousUTCDay(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()-1, 0, 0, 0, 0, time.UTC)
}

func untilNextUTCDay(now time.Time) time.Duration {
	utc := now.UTC()
	next := time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
	return next.Sub(utc)
}
