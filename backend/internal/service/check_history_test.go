package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const historyMonitorID = "11111111-1111-1111-1111-111111111111"

type fakeCheckHistoryMonitorRepository struct {
	err    error
	called bool
}

func (r *fakeCheckHistoryMonitorRepository) GetByID(context.Context, string) (*models.Monitor, error) {
	r.called = true
	if r.err != nil {
		return nil, r.err
	}
	return &models.Monitor{ID: historyMonitorID}, nil
}

type fakeCheckHistoryRepository struct {
	results []models.CheckResult
	err     error
	limit   int
	called  bool
}

func (r *fakeCheckHistoryRepository) ListByMonitorID(
	_ context.Context,
	_ string,
	limit int,
) ([]models.CheckResult, error) {
	r.called = true
	r.limit = limit
	return r.results, r.err
}

func TestCheckHistoryServiceListCheckHistory(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		limit      int
		monitorErr error
		wantErr    bool
		wantLimit  int
		wantCalls  bool
	}{
		{name: "returns empty history", id: historyMonitorID, limit: 50, wantLimit: 50, wantCalls: true},
		{name: "clamps maximum limit", id: historyMonitorID, limit: 500, wantLimit: 100, wantCalls: true},
		{name: "rejects zero limit", id: historyMonitorID, limit: 0, wantErr: true},
		{name: "rejects invalid monitor ID", id: "invalid", limit: 50, wantErr: true},
		{name: "propagates missing monitor", id: historyMonitorID, limit: 50, monitorErr: pgx.ErrNoRows, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitorRepo := &fakeCheckHistoryMonitorRepository{err: tt.monitorErr}
			checkLogRepo := &fakeCheckHistoryRepository{results: []models.CheckResult{}}
			svc := NewCheckHistoryService(monitorRepo, checkLogRepo)

			results, err := svc.ListCheckHistory(context.Background(), tt.id, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			if checkLogRepo.called != tt.wantCalls {
				t.Fatalf("check-log repository called = %v, want %v", checkLogRepo.called, tt.wantCalls)
			}
			if tt.wantCalls && checkLogRepo.limit != tt.wantLimit {
				t.Fatalf("limit = %d, want %d", checkLogRepo.limit, tt.wantLimit)
			}
			if tt.name == "returns empty history" && results == nil {
				t.Fatal("empty history should be an empty slice, not nil")
			}
			if errors.Is(err, pgx.ErrNoRows) != (tt.monitorErr == pgx.ErrNoRows) {
				t.Fatalf("missing monitor error wrapping mismatch: %v", err)
			}
		})
	}
}
