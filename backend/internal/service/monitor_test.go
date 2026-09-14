package service

import (
	"context"
	"errors"
	"testing"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type fakeMonitorRepository struct {
	createCalled bool
	createErr    error
}

func (f *fakeMonitorRepository) Create(
	ctx context.Context,
	monitor *models.Monitor,
) error {
	f.createCalled = true
	return f.createErr
}

func (f *fakeMonitorRepository) List(
	ctx context.Context,
) ([]models.Monitor, error) {
	return nil, nil
}

func (f *fakeMonitorRepository) GetByID(
	ctx context.Context,
	id string,
) (*models.Monitor, error) {
	return nil, nil
}

func (f *fakeMonitorRepository) Delete(
	ctx context.Context,
	id string,
) error {
	return nil
}

func validMonitor() *models.Monitor {
	return &models.Monitor{
		Name:               "Test Monitor",
		URL:                "https://example.com",
		ExpectedStatusCode: 200,
		IntervalSeconds:    30,
		TimeoutSeconds:     5,
	}
}

func TestMonitorService_CreateMonitor(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*models.Monitor)
		repoErr     error
		wantErr     bool
		wantRepoHit bool
	}{
		{
			name:        "valid monitor",
			wantErr:     false,
			wantRepoHit: true,
		},
		{
			name: "missing name",
			modify: func(m *models.Monitor) {
				m.Name = ""
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name: "missing URL",
			modify: func(m *models.Monitor) {
				m.URL = ""
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name: "invalid URL",
			modify: func(m *models.Monitor) {
				m.URL = "not-a-url"
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name: "unsupported URL scheme",
			modify: func(m *models.Monitor) {
				m.URL = "ftp://example.com"
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name: "invalid status code",
			modify: func(m *models.Monitor) {
				m.ExpectedStatusCode = 99
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name: "interval too short",
			modify: func(m *models.Monitor) {
				m.IntervalSeconds = 3
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name: "timeout too small",
			modify: func(m *models.Monitor) {
				m.TimeoutSeconds = 0
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name: "timeout too large",
			modify: func(m *models.Monitor) {
				m.TimeoutSeconds = 301
			},
			wantErr:     true,
			wantRepoHit: false,
		},
		{
			name:        "repository failure",
			repoErr:     errors.New("database failure"),
			wantErr:     true,
			wantRepoHit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeMonitorRepository{
				createErr: tt.repoErr,
			}

			service := NewMonitorService(repo)

			monitor := validMonitor()

			if tt.modify != nil {
				tt.modify(monitor)
			}

			err := service.CreateMonitor(
				context.Background(),
				monitor,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error: %v, got error: %v", tt.wantErr, err)
			}

			if repo.createCalled != tt.wantRepoHit {
				t.Fatalf(
					"expected repository called: %v, got: %v",
					tt.wantRepoHit,
					repo.createCalled,
				)
			}
		})
	}
}

func (f *fakeMonitorRepository) Update(
	ctx context.Context,
	monitor *models.Monitor,
) error {
	return nil
}
