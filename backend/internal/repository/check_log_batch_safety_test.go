package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createBatchTestMonitor(t *testing.T, pool *pgxpool.Pool) models.Monitor {
	t.Helper()
	monitor := models.Monitor{
		Name: "Batch safety test", URL: "https://example.com",
		ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5,
	}
	if err := NewMonitorRepository(pool).Create(context.Background(), &monitor); err != nil {
		t.Fatal(err)
	}
	return monitor
}

func TestCreateBatchSkipsDeletedMonitors(t *testing.T) {
	pool := newTestDatabase(t)
	deleted := createBatchTestMonitor(t, pool)
	survivor := createBatchTestMonitor(t, pool)
	ctx := context.Background()
	if err := NewMonitorRepository(pool).Delete(ctx, deleted.ID); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	results := []models.CheckResult{
		{MonitorID: deleted.ID, CheckedAt: at, Reachable: false},
		{MonitorID: strings.ToUpper(survivor.ID), CheckedAt: at, Reachable: true, StatusCode: 200},
		{MonitorID: survivor.ID, CheckedAt: at.Add(time.Second), Reachable: false, StatusCode: 500},
	}
	repo := NewCheckLogRepository(pool)
	persisted, err := repo.CreateBatch(ctx, results)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persisted, results[1:]) {
		t.Fatalf("committed results = %+v, want %+v", persisted, results[1:])
	}
	history, err := repo.ListByMonitorID(ctx, survivor.ID, 50)
	if err != nil || len(history) != 2 {
		t.Fatalf("surviving history = %+v, %v", history, err)
	}
	for _, input := range [][]models.CheckResult{nil, results[:1]} {
		persisted, err := repo.CreateBatch(ctx, input)
		if err != nil || len(persisted) != 0 {
			t.Fatalf("empty/deleted batch = %+v, %v", persisted, err)
		}
	}
}

func TestCreateBatchRollsBackAllRowsOnFailure(t *testing.T) {
	pool := newTestDatabase(t)
	monitor := createBatchTestMonitor(t, pool)
	ctx := context.Background()
	// A test-only constraint deterministically fails the second COPY row.
	if _, err := pool.Exec(ctx, "ALTER TABLE check_logs ADD CHECK (latency_ms >= 0)"); err != nil {
		t.Fatal(err)
	}
	results := []models.CheckResult{
		{MonitorID: monitor.ID, CheckedAt: time.Now().UTC(), LatencyMs: 10, Reachable: true},
		{MonitorID: monitor.ID, CheckedAt: time.Now().UTC(), LatencyMs: -1, Reachable: true},
	}
	repo := NewCheckLogRepository(pool)
	persisted, err := repo.CreateBatch(ctx, results)
	if err == nil || len(persisted) != 0 {
		t.Fatalf("failed batch reported committed rows: %+v, %v", persisted, err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM check_logs").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed batch left %d rows, error %v", count, err)
	}

	results[1].LatencyMs = 20
	persisted, err = repo.CreateBatch(ctx, results)
	if err != nil || len(persisted) != 2 {
		t.Fatalf("corrected batch = %+v, %v", persisted, err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM check_logs").Scan(&count); err != nil || count != 2 {
		t.Fatalf("corrected batch has %d rows, error %v; want exactly 2", count, err)
	}
}

func TestCreateBatchConcurrentDelete(t *testing.T) {
	for _, cancelBatch := range []bool{false, true} {
		name := "delete commits while batch waits"
		if cancelBatch {
			name = "cancellation releases waiting batch"
		}
		t.Run(name, func(t *testing.T) {
			pool := newTestDatabase(t)
			deleted := createBatchTestMonitor(t, pool)
			survivor := createBatchTestMonitor(t, pool)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			deletion, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer deletion.Rollback(context.Background())
			var deletionPID int32
			if err := deletion.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&deletionPID); err != nil {
				t.Fatal(err)
			}
			if _, err := deletion.Exec(ctx, "DELETE FROM monitors WHERE id = $1", deleted.ID); err != nil {
				t.Fatal(err)
			}

			type outcome struct {
				results []models.CheckResult
				err     error
			}
			batchCtx, cancelBatchCtx := context.WithCancel(ctx)
			defer cancelBatchCtx()
			done := make(chan outcome, 1)
			go func() {
				results, err := NewCheckLogRepository(pool).CreateBatch(batchCtx, []models.CheckResult{
					{MonitorID: deleted.ID, CheckedAt: time.Now().UTC()},
					{MonitorID: survivor.ID, CheckedAt: time.Now().UTC()},
				})
				done <- outcome{results, err}
			}()

			// Wait for an actual database lock, not an assumed goroutine delay.
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				var blocked bool
				err := pool.QueryRow(ctx, `
					SELECT EXISTS (
						SELECT 1 FROM pg_stat_activity
						WHERE $1::integer = ANY(pg_blocking_pids(pid))
					)
				`, deletionPID).Scan(&blocked)
				if err != nil {
					t.Fatalf("wait for batch lock: %v", err)
				}
				if blocked {
					break
				}
				select {
				case early := <-done:
					t.Fatalf("batch completed before deletion was resolved: %+v", early)
				case <-ctx.Done():
					t.Fatal("batch did not wait on the deleting transaction")
				case <-ticker.C:
				}
			}

			if cancelBatch {
				cancelBatchCtx()
			} else if err := deletion.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var result outcome
			select {
			case result = <-done:
			case <-ctx.Done():
				t.Fatal("batch did not finish")
			}
			if cancelBatch {
				if !errors.Is(result.err, context.Canceled) || len(result.results) != 0 {
					t.Fatalf("cancelled batch = %+v", result)
				}
				if err := deletion.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				var count int
				if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM check_logs").Scan(&count); err != nil || count != 0 {
					t.Fatalf("cancelled batch left %d rows, error %v", count, err)
				}
			} else {
				if result.err != nil || len(result.results) != 1 || result.results[0].MonitorID != survivor.ID {
					t.Fatalf("concurrent deletion discarded surviving results: %+v", result)
				}
				// Cascading deletion remains functional after the write releases its locks.
				if err := NewMonitorRepository(pool).Delete(ctx, survivor.ID); err != nil {
					t.Fatal(err)
				}
				var count int
				if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM check_logs").Scan(&count); err != nil || count != 0 {
					t.Fatalf("cascade left %d rows, error %v", count, err)
				}
			}
		})
	}
}
