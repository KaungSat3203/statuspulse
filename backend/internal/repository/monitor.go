package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type MonitorRepository struct {
	db *pgxpool.Pool
}

func NewMonitorRepository(db *pgxpool.Pool) *MonitorRepository {
	return &MonitorRepository{
		db: db,
	}
}

func (r *MonitorRepository) Create(
	ctx context.Context,
	monitor *models.Monitor,
) error {
	query := `
		INSERT INTO monitors (
			name,
			url,
			expected_status_code,
			interval_seconds,
			timeout_seconds
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			name,
			url,
			expected_status_code,
			interval_seconds,
			timeout_seconds,
			active,
			consecutive_failures,
			consecutive_successes,
			created_at,
			updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		monitor.Name,
		monitor.URL,
		monitor.ExpectedStatusCode,
		monitor.IntervalSeconds,
		monitor.TimeoutSeconds,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.URL,
		&monitor.ExpectedStatusCode,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Active,
		&monitor.ConsecutiveFailures,
		&monitor.ConsecutiveSuccesses,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("create monitor: %w", err)
	}

	return nil
}

func (r *MonitorRepository) List(
	ctx context.Context,
) ([]models.Monitor, error) {
	query := `
		SELECT
			id,
			name,
			url,
			expected_status_code,
			interval_seconds,
			timeout_seconds,
			active,
			consecutive_failures,
			consecutive_successes,
			created_at,
			updated_at
		FROM monitors
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list monitors: %w", err)
	}
	defer rows.Close()

	var monitors []models.Monitor

	for rows.Next() {
		var monitor models.Monitor

		err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.URL,
			&monitor.ExpectedStatusCode,
			&monitor.IntervalSeconds,
			&monitor.TimeoutSeconds,
			&monitor.Active,
			&monitor.ConsecutiveFailures,
			&monitor.ConsecutiveSuccesses,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan monitor: %w", err)
		}

		monitors = append(monitors, monitor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitors: %w", err)
	}

	return monitors, nil
}

func (r *MonitorRepository) GetByID(
	ctx context.Context,
	id string,
) (*models.Monitor, error) {
	query := `
		SELECT
			id,
			name,
			url,
			expected_status_code,
			interval_seconds,
			timeout_seconds,
			active,
			consecutive_failures,
			consecutive_successes,
			created_at,
			updated_at
		FROM monitors
		WHERE id = $1
	`

	var monitor models.Monitor

	err := r.db.QueryRow(
		ctx,
		query,
		id,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.URL,
		&monitor.ExpectedStatusCode,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Active,
		&monitor.ConsecutiveFailures,
		&monitor.ConsecutiveSuccesses,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get monitor: %w", err)
	}

	return &monitor, nil
}

func (r *MonitorRepository) Update(
	ctx context.Context,
	monitor *models.Monitor,
) error {
	query := `
		UPDATE monitors
		SET
			name = $2,
			url = $3,
			expected_status_code = $4,
			interval_seconds = $5,
			timeout_seconds = $6,
			active = $7,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			name,
			url,
			expected_status_code,
			interval_seconds,
			timeout_seconds,
			active,
			consecutive_failures,
			consecutive_successes,
			created_at,
			updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		monitor.ID,
		monitor.Name,
		monitor.URL,
		monitor.ExpectedStatusCode,
		monitor.IntervalSeconds,
		monitor.TimeoutSeconds,
		monitor.Active,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.URL,
		&monitor.ExpectedStatusCode,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Active,
		&monitor.ConsecutiveFailures,
		&monitor.ConsecutiveSuccesses,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update monitor: %w", err)
	}

	return nil
}
func (r *MonitorRepository) Delete(
	ctx context.Context,
	id string,
) error {
	query := `
		DELETE FROM monitors
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete monitor: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("monitor not found")
	}

	return nil
}

func (r *MonitorRepository) RecordCheckResult(
	ctx context.Context,
	result models.CheckResult,
) (*models.Monitor, error) {
	query := `
		UPDATE monitors
		SET
			consecutive_failures = CASE
				WHEN $2 THEN 0
				ELSE consecutive_failures + 1
			END,
			consecutive_successes = CASE
				WHEN $2 THEN consecutive_successes + 1
				ELSE 0
			END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			name,
			url,
			expected_status_code,
			interval_seconds,
			timeout_seconds,
			active,
			consecutive_failures,
			consecutive_successes,
			created_at,
			updated_at
	`

	monitor := &models.Monitor{}
	err := r.db.QueryRow(ctx, query, result.MonitorID, result.Reachable).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.URL,
		&monitor.ExpectedStatusCode,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Active,
		&monitor.ConsecutiveFailures,
		&monitor.ConsecutiveSuccesses,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("record check result: %w", err)
	}

	return monitor, nil
}
