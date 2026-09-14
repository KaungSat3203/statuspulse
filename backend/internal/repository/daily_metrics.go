package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type DailyMetricsRepository struct{ db *pgxpool.Pool }

func NewDailyMetricsRepository(db *pgxpool.Pool) *DailyMetricsRepository {
	return &DailyMetricsRepository{db: db}
}

func (r *DailyMetricsRepository) Aggregate(
	ctx context.Context,
	monitorID string,
	metricDate time.Time,
) (*models.DailyMetric, error) {
	query := `
		WITH aggregates AS (
			SELECT
				$1::uuid AS monitor_id,
				$2::date AS metric_date,
				COUNT(*)::bigint AS total_checks,
				COUNT(*) FILTER (WHERE reachable)::bigint AS successful_checks,
				AVG(latency_ms)::double precision AS mean_latency_ms,
				PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms)::double precision AS p95_latency_ms
			FROM check_logs
			WHERE monitor_id = $1
				AND checked_at >= $2::date
				AND checked_at < $2::date + INTERVAL '1 day'
		)
		INSERT INTO daily_metrics (
			monitor_id, metric_date, total_checks, successful_checks, mean_latency_ms, p95_latency_ms
		)
		SELECT monitor_id, metric_date, total_checks, successful_checks, mean_latency_ms, p95_latency_ms
		FROM aggregates
		ON CONFLICT (monitor_id, metric_date) DO UPDATE
		SET
			total_checks = EXCLUDED.total_checks,
			successful_checks = EXCLUDED.successful_checks,
			mean_latency_ms = EXCLUDED.mean_latency_ms,
			p95_latency_ms = EXCLUDED.p95_latency_ms
		RETURNING
			id, monitor_id, metric_date, total_checks, successful_checks,
			mean_latency_ms::double precision, p95_latency_ms::double precision, created_at
	`

	metric := &models.DailyMetric{}
	err := r.db.QueryRow(ctx, query, monitorID, metricDate.Format("2006-01-02")).Scan(
		&metric.ID, &metric.MonitorID, &metric.MetricDate, &metric.TotalChecks,
		&metric.SuccessfulChecks, &metric.MeanLatencyMs, &metric.P95LatencyMs, &metric.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate daily metrics: %w", err)
	}
	return metric, nil
}

func (r *DailyMetricsRepository) ListByMonitorID(
	ctx context.Context,
	monitorID string,
	limit int,
) ([]models.DailyMetric, error) {
	query := `
		SELECT
			monitor_id,
			metric_date,
			total_checks,
			successful_checks,
			mean_latency_ms,
			p95_latency_ms
		FROM daily_metrics
		WHERE monitor_id = $1
		ORDER BY metric_date DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list daily metrics: %w", err)
	}
	defer rows.Close()

	metrics := make([]models.DailyMetric, 0)
	for rows.Next() {
		var metric models.DailyMetric
		if err := rows.Scan(
			&metric.MonitorID,
			&metric.MetricDate,
			&metric.TotalChecks,
			&metric.SuccessfulChecks,
			&metric.MeanLatencyMs,
			&metric.P95LatencyMs,
		); err != nil {
			return nil, fmt.Errorf("scan daily metric: %w", err)
		}
		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily metrics: %w", err)
	}
	return metrics, nil
}
