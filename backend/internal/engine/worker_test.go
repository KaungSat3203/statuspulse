package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestWorkerPool_Run(t *testing.T) {
	monitors := []models.Monitor{
		{ID: "monitor-1"},
		{ID: "monitor-2"},
		{ID: "monitor-3"},
		{ID: "monitor-4"},
		{ID: "monitor-5"},
		{ID: "monitor-6"},
	}

	var active int32
	var maxActive int32

	fakeCheck := func(
		ctx context.Context,
		monitor models.Monitor,
	) models.CheckResult {
		current := atomic.AddInt32(&active, 1)

		for {
			old := atomic.LoadInt32(&maxActive)

			if current <= old {
				break
			}

			if atomic.CompareAndSwapInt32(&maxActive, old, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)

		atomic.AddInt32(&active, -1)

		return models.CheckResult{
			MonitorID: monitor.ID,
			Reachable: true,
		}
	}

	pool := NewWorkerPool(3, fakeCheck)

	results := pool.Run(context.Background(), monitors)

	if len(results) != len(monitors) {
		t.Fatalf(
			"expected %d results, got %d",
			len(monitors),
			len(results),
		)
	}

	if maxActive <= 1 {
		t.Fatalf(
			"expected checks to run concurrently, max active was %d",
			maxActive,
		)
	}

	if maxActive > 3 {
		t.Fatalf(
			"expected at most 3 concurrent checks, max active was %d",
			maxActive,
		)
	}
}
