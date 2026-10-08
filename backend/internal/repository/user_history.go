package repository

import (
	"context"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// History queries also enforce ownership themselves, not just in the preceding
// monitor-existence check. Ownership changes cannot bypass that boundary.
type UserCheckLogRepository struct{ db *pgxpool.Pool }

func NewUserCheckLogRepository(db *pgxpool.Pool) *UserCheckLogRepository {
	return &UserCheckLogRepository{db}
}
func (r *UserCheckLogRepository) ListByMonitorID(ctx context.Context, id string, limit int) ([]models.CheckResult, error) {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT c.monitor_id,c.checked_at,COALESCE(c.status_code,0),
        COALESCE(c.latency_ms,0),c.reachable,COALESCE(c.error_message,'')
        FROM check_logs c JOIN monitors m ON m.id=c.monitor_id
        WHERE c.monitor_id=$1 AND m.user_id=$2 ORDER BY c.checked_at DESC LIMIT $3`, id, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]models.CheckResult, 0)
	for rows.Next() {
		var c models.CheckResult
		if err := rows.Scan(&c.MonitorID, &c.CheckedAt, &c.StatusCode, &c.LatencyMs, &c.Reachable, &c.Error); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

type UserDailyMetricsRepository struct{ db *pgxpool.Pool }

func NewUserDailyMetricsRepository(db *pgxpool.Pool) *UserDailyMetricsRepository {
	return &UserDailyMetricsRepository{db}
}
func (r *UserDailyMetricsRepository) ListByMonitorID(ctx context.Context, id string, limit int) ([]models.DailyMetric, error) {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT d.monitor_id,d.metric_date,d.total_checks,d.successful_checks,
        d.mean_latency_ms,d.p95_latency_ms FROM daily_metrics d JOIN monitors m ON m.id=d.monitor_id
        WHERE d.monitor_id=$1 AND m.user_id=$2 ORDER BY d.metric_date DESC LIMIT $3`, id, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]models.DailyMetric, 0)
	for rows.Next() {
		var d models.DailyMetric
		if err := rows.Scan(&d.MonitorID, &d.MetricDate, &d.TotalChecks, &d.SuccessfulChecks, &d.MeanLatencyMs, &d.P95LatencyMs); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

type UserIncidentRepository struct{ db *pgxpool.Pool }

func NewUserIncidentRepository(db *pgxpool.Pool) *UserIncidentRepository {
	return &UserIncidentRepository{db}
}
func (r *UserIncidentRepository) ListRecent(ctx context.Context, limit int) ([]models.Incident, error) {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT i.id,i.monitor_id,i.started_at,i.recovered_at,i.downtime_seconds,
        COALESCE(i.reason,''),i.created_at FROM incidents i JOIN monitors m ON m.id=i.monitor_id
        WHERE m.user_id=$1 ORDER BY i.created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]models.Incident, 0)
	for rows.Next() {
		var i models.Incident
		if err := rows.Scan(&i.ID, &i.MonitorID, &i.StartedAt, &i.RecoveredAt, &i.DowntimeSeconds, &i.Reason, &i.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, i)
	}
	return results, rows.Err()
}
