package service

import (
	"context"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type fakeMonitorCheckStateRepository struct {
	monitor models.Monitor
}

func (f *fakeMonitorCheckStateRepository) RecordCheckResult(
	ctx context.Context,
	result models.CheckResult,
) (*models.Monitor, error) {
	if result.Reachable {
		f.monitor.ConsecutiveFailures = 0
		f.monitor.ConsecutiveSuccesses++
	} else {
		f.monitor.ConsecutiveFailures++
		f.monitor.ConsecutiveSuccesses = 0
	}

	return &f.monitor, nil
}

type fakeIncidentRepository struct {
	open          *models.Incident
	createCalls   int
	recoverCalls  int
	lastRecovered time.Time
	recent        []models.Incident
	listLimit     int
}

func (f *fakeIncidentRepository) Create(
	ctx context.Context,
	incident *models.Incident,
) error {
	f.createCalls++
	incident.ID = "incident-1"
	f.open = incident

	return nil
}

func (f *fakeIncidentRepository) ListRecent(
	ctx context.Context,
	limit int,
) ([]models.Incident, error) {
	f.listLimit = limit
	return f.recent, nil
}

func (f *fakeIncidentRepository) GetOpenByMonitorID(
	ctx context.Context,
	monitorID string,
) (*models.Incident, error) {
	return f.open, nil
}

func (f *fakeIncidentRepository) Recover(
	ctx context.Context,
	incidentID string,
	recoveredAt time.Time,
) error {
	f.recoverCalls++
	f.lastRecovered = recoveredAt
	f.open = nil

	return nil
}

func TestIncidentService_ProcessCheck(t *testing.T) {
	newService := func() (
		*IncidentService,
		*fakeIncidentRepository,
	) {
		monitorRepo := &fakeMonitorCheckStateRepository{
			monitor: models.Monitor{ID: "monitor-1"},
		}
		incidentRepo := &fakeIncidentRepository{}

		return NewIncidentService(monitorRepo, incidentRepo), incidentRepo
	}

	failedCheck := func(checkedAt time.Time) models.CheckResult {
		return models.CheckResult{
			MonitorID: "monitor-1",
			CheckedAt: checkedAt,
			Reachable: false,
		}
	}

	successfulCheck := func(checkedAt time.Time) models.CheckResult {
		return models.CheckResult{
			MonitorID: "monitor-1",
			CheckedAt: checkedAt,
			Reachable: true,
		}
	}

	t.Run("one failure does not create an incident", func(t *testing.T) {
		service, incidentRepo := newService()

		if err := service.ProcessCheck(
			context.Background(),
			failedCheck(time.Now()),
		); err != nil {
			t.Fatalf("ProcessCheck() error = %v", err)
		}

		if incidentRepo.createCalls != 0 {
			t.Fatalf("expected no incidents, got %d", incidentRepo.createCalls)
		}
	})

	t.Run("two failures do not create an incident", func(t *testing.T) {
		service, incidentRepo := newService()
		now := time.Now()

		for i := 0; i < 2; i++ {
			if err := service.ProcessCheck(
				context.Background(),
				failedCheck(now.Add(time.Duration(i)*time.Minute)),
			); err != nil {
				t.Fatalf("ProcessCheck() error = %v", err)
			}
		}

		if incidentRepo.createCalls != 0 {
			t.Fatalf("expected no incidents, got %d", incidentRepo.createCalls)
		}
	})

	t.Run("third failure creates exactly one incident", func(t *testing.T) {
		service, incidentRepo := newService()
		now := time.Now()

		for i := 0; i < 3; i++ {
			if err := service.ProcessCheck(
				context.Background(),
				failedCheck(now.Add(time.Duration(i)*time.Minute)),
			); err != nil {
				t.Fatalf("ProcessCheck() error = %v", err)
			}
		}

		if incidentRepo.createCalls != 1 {
			t.Fatalf("expected one incident, got %d", incidentRepo.createCalls)
		}
		if incidentRepo.open == nil {
			t.Fatal("expected an open incident")
		}
	})

	t.Run("fourth failure does not create a duplicate incident", func(t *testing.T) {
		service, incidentRepo := newService()
		now := time.Now()

		for i := 0; i < 4; i++ {
			if err := service.ProcessCheck(
				context.Background(),
				failedCheck(now.Add(time.Duration(i)*time.Minute)),
			); err != nil {
				t.Fatalf("ProcessCheck() error = %v", err)
			}
		}

		if incidentRepo.createCalls != 1 {
			t.Fatalf("expected one incident, got %d", incidentRepo.createCalls)
		}
	})

	t.Run("a success recovers an open incident once", func(t *testing.T) {
		service, incidentRepo := newService()
		now := time.Now()

		for i := 0; i < 3; i++ {
			if err := service.ProcessCheck(
				context.Background(),
				failedCheck(now.Add(time.Duration(i)*time.Minute)),
			); err != nil {
				t.Fatalf("ProcessCheck() error = %v", err)
			}
		}

		recoveredAt := now.Add(3 * time.Minute)
		if err := service.ProcessCheck(
			context.Background(),
			successfulCheck(recoveredAt),
		); err != nil {
			t.Fatalf("ProcessCheck() error = %v", err)
		}

		if incidentRepo.recoverCalls != 1 {
			t.Fatalf("expected one recovery, got %d", incidentRepo.recoverCalls)
		}
		if !incidentRepo.lastRecovered.Equal(recoveredAt) {
			t.Fatalf(
				"expected recovered at %v, got %v",
				recoveredAt,
				incidentRepo.lastRecovered,
			)
		}

		if err := service.ProcessCheck(
			context.Background(),
			successfulCheck(recoveredAt.Add(time.Minute)),
		); err != nil {
			t.Fatalf("ProcessCheck() error = %v", err)
		}

		if incidentRepo.recoverCalls != 1 {
			t.Fatalf(
				"expected no duplicate recovery, got %d recoveries",
				incidentRepo.recoverCalls,
			)
		}
	})

	t.Run("successful checks without an incident do nothing", func(t *testing.T) {
		service, incidentRepo := newService()

		if err := service.ProcessCheck(
			context.Background(),
			successfulCheck(time.Now()),
		); err != nil {
			t.Fatalf("ProcessCheck() error = %v", err)
		}

		if incidentRepo.createCalls != 0 {
			t.Fatalf("expected no created incidents, got %d", incidentRepo.createCalls)
		}
		if incidentRepo.recoverCalls != 0 {
			t.Fatalf("expected no recoveries, got %d", incidentRepo.recoverCalls)
		}
	})
}

func TestIncidentService_ListRecent(t *testing.T) {
	want := []models.Incident{{ID: "incident-1"}}
	repo := &fakeIncidentRepository{recent: want}
	service := NewIncidentService(
		&fakeMonitorCheckStateRepository{},
		repo,
	)

	got, err := service.ListRecent(context.Background(), 12)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if repo.listLimit != 12 {
		t.Fatalf("repository limit = %d, want 12", repo.listLimit)
	}
	if len(got) != 1 || got[0].ID != want[0].ID {
		t.Fatalf("ListRecent() = %+v, want %+v", got, want)
	}
}
