package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeCheckLogRetentionRepository struct {
	cutoffs []time.Time
	err     error
	called  chan struct{}
}

func (r *fakeCheckLogRetentionRepository) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	r.cutoffs = append(r.cutoffs, cutoff)
	if r.called != nil {
		select {
		case r.called <- struct{}{}:
		default:
		}
	}
	return 3, r.err
}

func TestCheckLogRetentionCutoff(t *testing.T) {
	now := time.Date(2026, time.January, 16, 2, 0, 0, 0, time.FixedZone("UTC+7", 7*3600))
	want := time.Date(2026, time.January, 8, 0, 0, 0, 0, time.UTC)
	if got := checkLogRetentionCutoff(now); !got.Equal(want) {
		t.Fatalf("cutoff = %v, want %v", got, want)
	}
}

func TestCheckLogRetentionJobRunOnce(t *testing.T) {
	now := time.Date(2026, time.January, 16, 10, 0, 0, 0, time.UTC)
	repo := &fakeCheckLogRetentionRepository{}
	job := NewCheckLogRetentionJobWithClock(repo, func() time.Time { return now }, func(time.Duration) dailyMetricsTimer {
		return &fakeDailyMetricsTimer{channel: make(chan time.Time)}
	})
	job.logf = func(string, ...any) {}

	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.cutoffs) != 1 {
		t.Fatalf("cleanup calls = %d, want 1", len(repo.cutoffs))
	}
	want := time.Date(2026, time.January, 9, 0, 0, 0, 0, time.UTC)
	if !repo.cutoffs[0].Equal(want) {
		t.Fatalf("cutoff = %v, want %v", repo.cutoffs[0], want)
	}
}

func TestCheckLogRetentionJobContinuesAfterCleanupError(t *testing.T) {
	now := time.Date(2026, time.January, 16, 10, 0, 0, 0, time.UTC)
	firstTimer := &fakeDailyMetricsTimer{channel: make(chan time.Time, 1)}
	firstTimer.channel <- now
	secondTimer := &fakeDailyMetricsTimer{channel: make(chan time.Time)}
	var timersMu sync.Mutex
	var timers []dailyMetricsTimer
	repo := &fakeCheckLogRetentionRepository{err: errors.New("database unavailable"), called: make(chan struct{}, 1)}
	job := NewCheckLogRetentionJobWithClock(repo, func() time.Time { return now }, func(time.Duration) dailyMetricsTimer {
		timersMu.Lock()
		defer timersMu.Unlock()
		if len(timers) == 0 {
			timers = append(timers, firstTimer)
			return firstTimer
		}
		timers = append(timers, secondTimer)
		return secondTimer
	})
	job.logf = func(string, ...any) {}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		job.Run(ctx)
		close(done)
	}()

	select {
	case <-repo.called:
	case <-time.After(time.Second):
		t.Fatal("cleanup was not called")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job did not stop after cancellation")
	}
	if !secondTimer.stopped {
		t.Fatal("waiting timer was not stopped after cancellation")
	}
}

func TestCheckLogRetentionJobSchedulingAndCancellation(t *testing.T) {
	now := time.Date(2026, time.January, 16, 10, 0, 0, 0, time.UTC)
	timer := &fakeDailyMetricsTimer{channel: make(chan time.Time)}
	repo := &fakeCheckLogRetentionRepository{}
	job := NewCheckLogRetentionJobWithClock(repo, func() time.Time { return now }, func(delay time.Duration) dailyMetricsTimer {
		if delay != 14*time.Hour {
			t.Errorf("delay = %v, want 14h", delay)
		}
		return timer
	})
	job.logf = func(string, ...any) {}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		job.Run(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job did not stop after cancellation")
	}
	if !timer.stopped {
		t.Fatal("timer was not stopped")
	}
}
