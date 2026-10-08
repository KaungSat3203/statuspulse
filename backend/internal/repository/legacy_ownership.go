package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// AssignLegacyMonitor is an operator-only operation, never exposed by HTTP.
// Dry runs take the same locks/checks but roll back without changing data.
func (r *MonitorRepository) AssignLegacyMonitor(ctx context.Context, monitorID, userID string, apply bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var existingUser string
	if err := tx.QueryRow(ctx, "SELECT id FROM users WHERE id=$1 FOR KEY SHARE", userID).Scan(&existingUser); err != nil {
		return err
	}
	var owner *string
	if err := tx.QueryRow(ctx, "SELECT user_id::text FROM monitors WHERE id=$1 FOR UPDATE", monitorID).Scan(&owner); err != nil {
		return err
	}
	if owner != nil {
		return errors.New("monitor already has an owner")
	}
	if !apply {
		return nil
	}
	result, err := tx.Exec(ctx, "UPDATE monitors SET user_id=$2,updated_at=NOW() WHERE id=$1 AND user_id IS NULL", monitorID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}
