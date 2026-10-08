package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CheckLogRepository struct {
	db *pgxpool.Pool
}

func NewCheckLogRepository(db *pgxpool.Pool) *CheckLogRepository {
	return &CheckLogRepository{db: db}
}

// ListByMonitorID returns the newest check logs for a monitor first.
func (r *CheckLogRepository) ListByMonitorID(
	ctx context.Context,
	monitorID string,
	limit int,
) ([]models.CheckResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			monitor_id,
			checked_at,
			COALESCE(status_code, 0),
			COALESCE(latency_ms, 0),
			reachable,
			COALESCE(error_message, '')
		FROM check_logs
		WHERE monitor_id = $1
		ORDER BY checked_at DESC
		LIMIT $2
	`, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list check logs: %w", err)
	}
	defer rows.Close()

	results := make([]models.CheckResult, 0)
	for rows.Next() {
		var result models.CheckResult
		if err := rows.Scan(
			&result.MonitorID,
			&result.CheckedAt,
			&result.StatusCode,
			&result.LatencyMs,
			&result.Reachable,
			&result.Error,
		); err != nil {
			return nil, fmt.Errorf("scan check log: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate check logs: %w", err)
	}
	return results, nil
}

// DeleteOlderThan removes raw check logs before cutoff. Logs exactly at the
// cutoff remain, so callers can retain complete calendar-day boundaries.
func (r *CheckLogRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.Exec(ctx, `DELETE FROM check_logs WHERE checked_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete check logs older than %s: %w", cutoff.Format(time.RFC3339), err)
	}
	return result.RowsAffected(), nil
}

// CreateBatch atomically saves results whose monitors still exist and returns
// only committed results. Parent-row locks close the race between filtering and
// COPY; a concurrent DELETE waits for commit, then cascades normally.
func (r *CheckLogRepository) CreateBatch(
	ctx context.Context,
	results []models.CheckResult,
) ([]models.CheckResult, error) {
	if len(results) == 0 {
		return nil, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin check log batch: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	ids := make([]string, len(results))
	for i, result := range results {
		ids[i] = result.MonitorID
	}
	// Always acquire locks in the same order for overlapping batches.
	monitors, err := tx.Query(ctx, `
		SELECT id FROM monitors
		WHERE id = ANY($1::uuid[])
		ORDER BY id
		FOR KEY SHARE
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("lock check log monitors: %w", err)
	}
	existing := make(map[string]struct{}, len(results))
	for monitors.Next() {
		var id string
		if err := monitors.Scan(&id); err != nil {
			monitors.Close()
			return nil, fmt.Errorf("scan check log monitor: %w", err)
		}
		existing[id] = struct{}{}
	}
	err = monitors.Err()
	monitors.Close()
	if err != nil {
		return nil, fmt.Errorf("iterate check log monitors: %w", err)
	}

	persisted := make([]models.CheckResult, 0, len(results))
	rows := make([][]any, 0, len(results))
	for _, result := range results {
		if _, ok := existing[strings.ToLower(result.MonitorID)]; !ok {
			continue
		}
		persisted = append(persisted, result)
		rows = append(rows, []any{
			result.MonitorID, result.CheckedAt, result.StatusCode,
			result.LatencyMs, result.Reachable, result.Error,
		})
	}
	if len(rows) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"check_logs"}, []string{
			"monitor_id", "checked_at", "status_code", "latency_ms", "reachable", "error_message",
		}, pgx.CopyFromRows(rows)); err != nil {
			return nil, fmt.Errorf("copy check log batch: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit check log batch: %w", err)
	}
	return persisted, nil
}
