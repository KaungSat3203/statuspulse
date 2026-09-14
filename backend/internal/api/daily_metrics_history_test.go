package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const dailyMetricsHistoryMonitorID = "22222222-2222-2222-2222-222222222222"

type fakeDailyMetricsHistoryReadService struct {
	metrics []models.DailyMetric
	err     error
	limit   int
}

func (s *fakeDailyMetricsHistoryReadService) ListMonitorMetrics(
	_ context.Context,
	_ string,
	limit int,
) ([]models.DailyMetric, error) {
	s.limit = limit
	return s.metrics, s.err
}

func TestDailyMetricsHistoryHandlerListMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		path      string
		service   *fakeDailyMetricsHistoryReadService
		wantCode  int
		wantLimit int
	}{
		{
			name:      "uses default limit",
			path:      "/monitors/" + dailyMetricsHistoryMonitorID + "/metrics",
			service:   &fakeDailyMetricsHistoryReadService{metrics: []models.DailyMetric{}},
			wantCode:  http.StatusOK,
			wantLimit: 30,
		},
		{
			name:      "uses requested limit",
			path:      "/monitors/" + dailyMetricsHistoryMonitorID + "/metrics?limit=14",
			service:   &fakeDailyMetricsHistoryReadService{metrics: []models.DailyMetric{{MonitorID: dailyMetricsHistoryMonitorID, MetricDate: time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)}}},
			wantCode:  http.StatusOK,
			wantLimit: 14,
		},
		{
			name:      "passes high limit to service for clamping",
			path:      "/monitors/" + dailyMetricsHistoryMonitorID + "/metrics?limit=500",
			service:   &fakeDailyMetricsHistoryReadService{metrics: []models.DailyMetric{}},
			wantCode:  http.StatusOK,
			wantLimit: 365,
		},
		{
			name:     "rejects zero limit",
			path:     "/monitors/" + dailyMetricsHistoryMonitorID + "/metrics?limit=0",
			service:  &fakeDailyMetricsHistoryReadService{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "rejects non numeric limit",
			path:     "/monitors/" + dailyMetricsHistoryMonitorID + "/metrics?limit=bad",
			service:  &fakeDailyMetricsHistoryReadService{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "rejects invalid UUID",
			path:     "/monitors/not-a-uuid/metrics",
			service:  &fakeDailyMetricsHistoryReadService{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "returns not found for missing monitor",
			path:     "/monitors/" + dailyMetricsHistoryMonitorID + "/metrics",
			service:  &fakeDailyMetricsHistoryReadService{err: pgx.ErrNoRows},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/monitors/:id/metrics", NewDailyMetricsHistoryHandler(tt.service).ListMetrics)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantCode)
			}
			if tt.wantCode == http.StatusOK && tt.service.limit != tt.wantLimit {
				t.Fatalf("limit = %d, want %d", tt.service.limit, tt.wantLimit)
			}
			if tt.name == "uses default limit" && recorder.Body.String() != "[]" {
				t.Fatalf("empty response = %q, want []", recorder.Body.String())
			}
		})
	}
}
