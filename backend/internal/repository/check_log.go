package repository

import (
	"context"
	"fmt"
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

func (r *CheckLogRepository) CreateBatch(ctx context.Context, results []models.CheckResult) error {
	if len(results) == 0 {
		return nil
	}
	rows := make([][]interface{}, len(results))
	for i, result := range results {
		rows[i] = []interface{}{result.MonitorID, result.CheckedAt, result.StatusCode, result.LatencyMs, result.Reachable, result.Error}
	}
	_, err := r.db.CopyFrom(ctx, pgx.Identifier{"check_logs"}, []string{
		"monitor_id", "checked_at", "status_code", "latency_ms", "reachable", "error_message",
	}, pgx.CopyFromRows(rows))
	return err
}
