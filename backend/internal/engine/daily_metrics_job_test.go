package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type fakeDailyMetricsMonitorRepository struct{ monitors []models.Monitor }

func (r *fakeDailyMetricsMonitorRepository) List(context.Context) ([]models.Monitor, error) {
	return r.monitors, nil
}

type aggregateCall struct {
	monitorID string
	date      time.Time
}
type fakeDailyMetricsAggregator struct {
	calls    []aggregateCall
	failures map[string]error
	onCall   func()
}

func (a *fakeDailyMetricsAggregator) AggregateMonitorDate(_ context.Context, monitorID string, date time.Time) (*models.DailyMetric, error) {
	a.calls = append(a.calls, aggregateCall{monitorID, date})
	if a.onCall != nil {
		a.onCall()
	}
	return &models.DailyMetric{}, a.failures[monitorID]
}

type fakeDailyMetricsTimer struct {
	channel chan time.Time
	stopped bool
	mu      sync.Mutex
}

func (t *fakeDailyMetricsTimer) Chan() <-chan time.Time { return t.channel }
func (t *fakeDailyMetricsTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	return true
}

func TestPreviousUTCDay(t *testing.T) {
	now := time.Date(2026, time.January, 1, 8, 0, 0, 0, time.FixedZone("UTC+7", 7*3600))
	want := time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
	if got := previousUTCDay(now); !got.Equal(want) {
		t.Fatalf("previous day = %v, want %v", got, want)
	}
}

func TestDailyMetricsJobRunOnce(t *testing.T) {
	now := time.Date(2026, time.January, 16, 10, 0, 0, 0, time.UTC)
	repo := &fakeDailyMetricsMonitorRepository{monitors: []models.Monitor{{ID: "one"}, {ID: "two"}, {ID: "three"}}}
	aggregator := &fakeDailyMetricsAggregator{failures: map[string]error{"two": errors.New("failed")}}
	job := NewDailyMetricsJobWithClock(repo, aggregator, func() time.Time { return now }, func(time.Duration) dailyMetricsTimer { return &fakeDailyMetricsTimer{channel: make(chan time.Time)} })
	job.logf = func(string, ...any) {}
	if err := job.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(aggregator.calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(aggregator.calls))
	}
	wantDate := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	for _, call := range aggregator.calls {
		if !call.date.Equal(wantDate) {
			t.Fatalf("metric date = %v, want %v", call.date, wantDate)
		}
	}
}

func TestDailyMetricsJobSchedulingAndCancellation(t *testing.T) {
	now := time.Date(2026, time.January, 16, 10, 0, 0, 0, time.UTC)
	timer := &fakeDailyMetricsTimer{channel: make(chan time.Time)}
	job := NewDailyMetricsJobWithClock(&fakeDailyMetricsMonitorRepository{}, &fakeDailyMetricsAggregator{}, func() time.Time { return now }, func(delay time.Duration) dailyMetricsTimer {
		if delay != 14*time.Hour {
			t.Errorf("delay = %v, want 14h", delay)
		}
		return timer
	})
	job.logf = func(string, ...any) {}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { job.Run(ctx); close(done) }()
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
