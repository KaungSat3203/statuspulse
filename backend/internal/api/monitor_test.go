package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/KaungSat3203/statuspulse/backend/internal/service"
)

type monitorHandlerRepository struct {
	monitor   *models.Monitor
	deleteErr error
}

func (r *monitorHandlerRepository) Create(context.Context, *models.Monitor) error  { return nil }
func (r *monitorHandlerRepository) List(context.Context) ([]models.Monitor, error) { return nil, nil }
func (r *monitorHandlerRepository) GetByID(_ context.Context, id string) (*models.Monitor, error) {
	if r.monitor == nil || r.monitor.ID != id {
		return nil, pgx.ErrNoRows
	}
	return r.monitor, nil
}
func (r *monitorHandlerRepository) Update(_ context.Context, monitor *models.Monitor) error {
	r.monitor = monitor
	return nil
}
func (r *monitorHandlerRepository) Delete(context.Context, string) error { return r.deleteErr }

func TestMonitorHandlerUpdateMonitor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const monitorID = "11111111-1111-1111-1111-111111111111"
	tests := []struct {
		name, body string
		monitor    *models.Monitor
		wantStatus int
		wantName   string
		wantActive bool
	}{
		{"pauses monitor", `{"active":false}`, &models.Monitor{ID: monitorID, Name: "Monitor", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5, Active: true}, http.StatusOK, "Monitor", false},
		{"updates monitor configuration", `{"name":"API","url":"https://api.example.com","expected_status_code":201,"interval_seconds":60,"timeout_seconds":10}`, &models.Monitor{ID: monitorID, Name: "Monitor", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5, Active: true}, http.StatusOK, "API", true},
		{"rejects invalid interval", `{"interval_seconds":4}`, &models.Monitor{ID: monitorID, Name: "Monitor", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5, Active: true}, http.StatusBadRequest, "", false},
		{"rejects empty update", `{}`, &models.Monitor{ID: monitorID, Name: "Monitor", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5}, http.StatusBadRequest, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &monitorHandlerRepository{monitor: tt.monitor}
			router := gin.New()
			router.PATCH("/monitors/:id", NewMonitorHandler(service.NewMonitorService(repo)).UpdateMonitor)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPatch, "/monitors/"+monitorID, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				var got models.Monitor
				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Name != tt.wantName || got.Active != tt.wantActive {
					t.Fatalf("updated monitor = %+v", got)
				}
			}
		})
	}
}

func TestMonitorHandlerDeleteMonitor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const monitorID = "11111111-1111-1111-1111-111111111111"

	tests := []struct {
		name, id string
		repoErr  error
		want     int
	}{
		{name: "deletes monitor", id: monitorID, want: http.StatusNoContent},
		{name: "rejects invalid UUID", id: "invalid", want: http.StatusBadRequest},
		{name: "returns not found", id: monitorID, repoErr: pgx.ErrNoRows, want: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &monitorHandlerRepository{deleteErr: tt.repoErr}
			router := gin.New()
			router.DELETE("/monitors/:id", NewMonitorHandler(service.NewMonitorService(repo)).DeleteMonitor)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/monitors/"+tt.id, nil))
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}
}
