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

const checkHistoryMonitorID = "11111111-1111-1111-1111-111111111111"

type fakeCheckHistoryReadService struct {
	results []models.CheckResult
	err     error
	limit   int
}

func (s *fakeCheckHistoryReadService) ListCheckHistory(
	_ context.Context,
	_ string,
	limit int,
) ([]models.CheckResult, error) {
	s.limit = limit
	return s.results, s.err
}

func TestCheckHistoryHandlerListChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		path      string
		service   *fakeCheckHistoryReadService
		wantCode  int
		wantLimit int
	}{
		{
			name:      "uses default limit",
			path:      "/monitors/" + checkHistoryMonitorID + "/checks",
			service:   &fakeCheckHistoryReadService{results: []models.CheckResult{}},
			wantCode:  http.StatusOK,
			wantLimit: 50,
		},
		{
			name:      "uses requested limit",
			path:      "/monitors/" + checkHistoryMonitorID + "/checks?limit=20",
			service:   &fakeCheckHistoryReadService{results: []models.CheckResult{{MonitorID: checkHistoryMonitorID, CheckedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)}}},
			wantCode:  http.StatusOK,
			wantLimit: 20,
		},
		{
			name:      "passes high limit to service for clamping",
			path:      "/monitors/" + checkHistoryMonitorID + "/checks?limit=500",
			service:   &fakeCheckHistoryReadService{results: []models.CheckResult{}},
			wantCode:  http.StatusOK,
			wantLimit: 500,
		},
		{
			name:     "rejects zero limit",
			path:     "/monitors/" + checkHistoryMonitorID + "/checks?limit=0",
			service:  &fakeCheckHistoryReadService{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "rejects non numeric limit",
			path:     "/monitors/" + checkHistoryMonitorID + "/checks?limit=bad",
			service:  &fakeCheckHistoryReadService{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "rejects invalid UUID",
			path:     "/monitors/not-a-uuid/checks",
			service:  &fakeCheckHistoryReadService{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "returns not found for missing monitor",
			path:     "/monitors/" + checkHistoryMonitorID + "/checks",
			service:  &fakeCheckHistoryReadService{err: pgx.ErrNoRows},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/monitors/:id/checks", NewCheckHistoryHandler(tt.service).ListChecks)
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
