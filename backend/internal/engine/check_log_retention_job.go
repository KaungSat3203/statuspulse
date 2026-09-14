package engine

import (
	"context"
	"fmt"
	"log"
	"time"
)

const checkLogRetentionDays = 7

type CheckLogRetentionRepository interface {
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// CheckLogRetentionJob removes raw logs older than seven complete UTC days.
type CheckLogRetentionJob struct {
	checkLogRepo CheckLogRetentionRepository
	now          func() time.Time
	newTimer     func(time.Duration) dailyMetricsTimer
	logf         func(string, ...any)
}

func NewCheckLogRetentionJob(checkLogRepo CheckLogRetentionRepository) *CheckLogRetentionJob {
	return NewCheckLogRetentionJobWithClock(checkLogRepo, time.Now, func(d time.Duration) dailyMetricsTimer {
		return standardDailyMetricsTimer{timer: time.NewTimer(d)}
	})
}

func NewCheckLogRetentionJobWithClock(
	checkLogRepo CheckLogRetentionRepository,
	now func() time.Time,
	newTimer func(time.Duration) dailyMetricsTimer,
) *CheckLogRetentionJob {
	return &CheckLogRetentionJob{
		checkLogRepo: checkLogRepo,
		now:          now,
		newTimer:     newTimer,
		logf:         log.Printf,
	}
}

func (j *CheckLogRetentionJob) Run(ctx context.Context) {
	for {
		timer := j.newTimer(untilNextUTCDay(j.now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.Chan():
		}

		if err := j.RunOnce(ctx); err != nil && ctx.Err() == nil {
			j.logf("check log retention failed: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (j *CheckLogRetentionJob) RunOnce(ctx context.Context) error {
	cutoff := checkLogRetentionCutoff(j.now())
	deleted, err := j.checkLogRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("delete check logs older than %s: %w", cutoff.Format(time.RFC3339), err)
	}

	j.logf("deleted %d check logs older than %s", deleted, cutoff.Format(time.RFC3339))
	return nil
}

func checkLogRetentionCutoff(now time.Time) time.Time {
	utc := now.UTC()
	currentDay := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return currentDay.AddDate(0, 0, -checkLogRetentionDays)
}
