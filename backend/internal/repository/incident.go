package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type IncidentRepository struct {
	db *pgxpool.Pool
}

func NewIncidentRepository(db *pgxpool.Pool) *IncidentRepository {
	return &IncidentRepository{db: db}
}

func (r *IncidentRepository) Create(
	ctx context.Context,
	incident *models.Incident,
) error {
	query := `
		INSERT INTO incidents (monitor_id, started_at, reason)
		VALUES ($1, $2, $3)
		RETURNING id, monitor_id, started_at, reason, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		incident.MonitorID,
		incident.StartedAt,
		incident.Reason,
	).Scan(
		&incident.ID,
		&incident.MonitorID,
		&incident.StartedAt,
		&incident.Reason,
		&incident.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create incident: %w", err)
	}

	return nil
}

func (r *IncidentRepository) GetOpenByMonitorID(
	ctx context.Context,
	monitorID string,
) (*models.Incident, error) {
	query := `
		SELECT id, monitor_id, started_at, reason, created_at
		FROM incidents
		WHERE monitor_id = $1 AND recovered_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`

	incident := &models.Incident{}
	err := r.db.QueryRow(ctx, query, monitorID).Scan(
		&incident.ID,
		&incident.MonitorID,
		&incident.StartedAt,
		&incident.Reason,
		&incident.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get open incident: %w", err)
	}

	return incident, nil
}

func (r *IncidentRepository) Recover(
	ctx context.Context,
	incidentID string,
	recoveredAt time.Time,
) error {
	query := `
		UPDATE incidents
		SET
			recovered_at = $2,
			downtime_seconds = EXTRACT(EPOCH FROM ($2 - started_at))::BIGINT
		WHERE id = $1 AND recovered_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, incidentID, recoveredAt)
	if err != nil {
		return fmt.Errorf("recover incident: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("open incident not found")
	}

	return nil
}

func (r *IncidentRepository) ListRecent(
	ctx context.Context,
	limit int,
) ([]models.Incident, error) {
	query := `
		SELECT
			id,
			monitor_id,
			started_at,
			recovered_at,
			downtime_seconds,
			COALESCE(reason, ''),
			created_at
		FROM incidents
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent incidents: %w", err)
	}
	defer rows.Close()

	incidents := make([]models.Incident, 0)
	for rows.Next() {
		var incident models.Incident

		if err := rows.Scan(
			&incident.ID,
			&incident.MonitorID,
			&incident.StartedAt,
			&incident.RecoveredAt,
			&incident.DowntimeSeconds,
			&incident.Reason,
			&incident.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}

		incidents = append(incidents, incident)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incidents: %w", err)
	}

	return incidents, nil
}
