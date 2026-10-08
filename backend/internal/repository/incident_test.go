package repository

import (
	"context"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestIncidentRepository_Lifecycle(t *testing.T) {
	pool := newTestDatabase(t)

	ctx := context.Background()
	monitorRepo := NewMonitorRepository(pool)
	monitor := &models.Monitor{
		Name:               "Incident Repository Test Monitor",
		URL:                "https://example.com",
		ExpectedStatusCode: 200,
		IntervalSeconds:    30,
		TimeoutSeconds:     5,
	}

	if err := monitorRepo.Create(ctx, monitor); err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	repo := NewIncidentRepository(pool)
	startedAt := time.Now().Add(-2 * time.Minute).UTC().Truncate(time.Microsecond)
	incident := &models.Incident{
		MonitorID: monitor.ID,
		StartedAt: startedAt,
		Reason:    "monitor check failed",
	}

	if err := repo.Create(ctx, incident); err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if incident.ID == "" {
		t.Fatal("expected incident ID to be populated")
	}

	openIncident, err := repo.GetOpenByMonitorID(ctx, monitor.ID)
	if err != nil {
		t.Fatalf("get open incident: %v", err)
	}
	if openIncident == nil {
		t.Fatal("expected an open incident")
	}
	if openIncident.ID != incident.ID {
		t.Fatalf("expected incident ID %q, got %q", incident.ID, openIncident.ID)
	}

	recoveredAt := startedAt.Add(90 * time.Second)
	if err := repo.Recover(ctx, incident.ID, recoveredAt); err != nil {
		t.Fatalf("recover incident: %v", err)
	}

	openIncident, err = repo.GetOpenByMonitorID(ctx, monitor.ID)
	if err != nil {
		t.Fatalf("get open incident after recovery: %v", err)
	}
	if openIncident != nil {
		t.Fatal("expected no open incident after recovery")
	}

	var (
		storedRecoveredAt time.Time
		downtimeSeconds   int64
	)
	err = pool.QueryRow(
		ctx,
		`
		SELECT recovered_at, downtime_seconds
		FROM incidents
		WHERE id = $1
		`,
		incident.ID,
	).Scan(&storedRecoveredAt, &downtimeSeconds)
	if err != nil {
		t.Fatalf("read recovered incident: %v", err)
	}

	if !storedRecoveredAt.Equal(recoveredAt) {
		t.Fatalf(
			"expected recovered at %v, got %v",
			recoveredAt,
			storedRecoveredAt,
		)
	}
	if downtimeSeconds != 90 {
		t.Fatalf("expected downtime 90 seconds, got %d", downtimeSeconds)
	}

	incidents, err := repo.ListRecent(ctx, 100)
	if err != nil {
		t.Fatalf("list recent incidents: %v", err)
	}

	var listed *models.Incident
	for i := range incidents {
		if incidents[i].ID == incident.ID {
			listed = &incidents[i]
			break
		}
	}
	if listed == nil {
		t.Fatalf("expected recovered incident %q in recent incidents", incident.ID)
	}
	if listed.RecoveredAt == nil || !listed.RecoveredAt.Equal(recoveredAt) {
		t.Fatalf("expected recovered_at %v, got %+v", recoveredAt, listed.RecoveredAt)
	}
	if listed.DowntimeSeconds == nil || *listed.DowntimeSeconds != 90 {
		t.Fatalf("expected downtime 90 seconds, got %+v", listed.DowntimeSeconds)
	}
}
