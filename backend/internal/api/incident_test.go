package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

type fakeIncidentReadService struct {
	incidents []models.Incident
	limit     int
}

func (f *fakeIncidentReadService) ListRecent(
	ctx context.Context,
	limit int,
) ([]models.Incident, error) {
	f.limit = limit
	return f.incidents, nil
}

func TestIncidentHandlerListIncidents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		path      string
		wantLimit int
	}{
		{name: "uses default limit", path: "/incidents", wantLimit: 20},
		{name: "uses requested limit", path: "/incidents?limit=7", wantLimit: 7},
		{name: "clamps low limit", path: "/incidents?limit=0", wantLimit: 1},
		{name: "clamps high limit", path: "/incidents?limit=500", wantLimit: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeIncidentReadService{incidents: []models.Incident{{
				ID:        "incident-1",
				MonitorID: "monitor-1",
				StartedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			}}}
			router := gin.New()
			router.GET("/incidents", NewIncidentHandler(service).ListIncidents)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if service.limit != tt.wantLimit {
				t.Fatalf("limit = %d, want %d", service.limit, tt.wantLimit)
			}
			if body := recorder.Body.String(); body == "" || body == "null" {
				t.Fatalf("expected incident JSON response, got %q", body)
			}
		})
	}
}

func TestIncidentHandlerRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/incidents", NewIncidentHandler(&fakeIncidentReadService{}).ListIncidents)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/incidents?limit=invalid", nil),
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
