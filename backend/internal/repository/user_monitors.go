package repository

import (
	"context"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserMonitorRepository is exclusively for authenticated API services.
// The engine keeps MonitorRepository. A missing identity never falls back to
// global access, and ownership is part of every read/write SQL statement.
type UserMonitorRepository struct{ db *pgxpool.Pool }

func NewUserMonitorRepository(db *pgxpool.Pool) *UserMonitorRepository {
	return &UserMonitorRepository{db: db}
}

const userMonitorColumns = `id,name,url,expected_status_code,interval_seconds,timeout_seconds,
    active,consecutive_failures,consecutive_successes,created_at,updated_at`

func scanUserMonitor(row pgx.Row, monitor *models.Monitor) error {
	return row.Scan(&monitor.ID, &monitor.Name, &monitor.URL, &monitor.ExpectedStatusCode,
		&monitor.IntervalSeconds, &monitor.TimeoutSeconds, &monitor.Active, &monitor.ConsecutiveFailures,
		&monitor.ConsecutiveSuccesses, &monitor.CreatedAt, &monitor.UpdatedAt)
}

func (r *UserMonitorRepository) Create(ctx context.Context, monitor *models.Monitor) error {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return err
	}
	return scanUserMonitor(r.db.QueryRow(ctx, `INSERT INTO monitors
        (user_id,name,url,expected_status_code,interval_seconds,timeout_seconds) VALUES($1,$2,$3,$4,$5,$6)
        RETURNING `+userMonitorColumns, userID, monitor.Name, monitor.URL, monitor.ExpectedStatusCode,
		monitor.IntervalSeconds, monitor.TimeoutSeconds), monitor)
}

func (r *UserMonitorRepository) List(ctx context.Context) ([]models.Monitor, error) {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+userMonitorColumns+` FROM monitors WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	monitors := make([]models.Monitor, 0)
	for rows.Next() {
		var m models.Monitor
		if err := scanUserMonitor(rows, &m); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, rows.Err()
}
func (r *UserMonitorRepository) GetByID(ctx context.Context, id string) (*models.Monitor, error) {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return nil, err
	}
	m := &models.Monitor{}
	err = scanUserMonitor(r.db.QueryRow(ctx, `SELECT `+userMonitorColumns+` FROM monitors WHERE id=$1 AND user_id=$2`, id, userID), m)
	return m, err
}
func (r *UserMonitorRepository) Update(ctx context.Context, m *models.Monitor) error {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return err
	}
	return scanUserMonitor(r.db.QueryRow(ctx, `UPDATE monitors SET name=$3,url=$4,expected_status_code=$5,
        interval_seconds=$6,timeout_seconds=$7,active=$8,updated_at=NOW() WHERE id=$1 AND user_id=$2
        RETURNING `+userMonitorColumns, m.ID, userID, m.Name, m.URL, m.ExpectedStatusCode, m.IntervalSeconds, m.TimeoutSeconds, m.Active), m)
}
func (r *UserMonitorRepository) Delete(ctx context.Context, id string) error {
	userID, err := auth.UserID(ctx)
	if err != nil {
		return err
	}
	result, err := r.db.Exec(ctx, `DELETE FROM monitors WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
