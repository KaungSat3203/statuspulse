package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/telemetry"
)

func TestEventHandlerCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{
			name:       "allows local frontend origin",
			origin:     "http://localhost:3000",
			wantOrigin: "http://localhost:3000",
		},
		{
			name:       "does not allow unrelated origin",
			origin:     "http://example.com",
			wantOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			request := httptest.NewRequest(
				http.MethodGet,
				"/events",
				nil,
			).WithContext(ctx)
			request.Header.Set("Origin", tt.origin)

			recorder := httptest.NewRecorder()
			router := gin.New()
			h := NewAuthHandler(&fakeAuth{allowed: true}, auth.Settings{Origins: []string{localFrontendOrigin}})
			router.Use(h.BrowserSecurity(), h.Authenticate())
			request.AddCookie(&http.Cookie{Name: h.settings.CookieName(), Value: "opaque-cookie"})
			router.GET(
				"/events",
				NewEventHandler(telemetry.NewBroadcaster(), h, nil).Stream,
			)

			router.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
				t.Fatalf(
					"Access-Control-Allow-Origin = %q, want %q",
					got,
					tt.wantOrigin,
				)
			}
			if tt.wantOrigin == "" {
				if recorder.Code != http.StatusForbidden {
					t.Fatal("untrusted origin accepted")
				}
				return
			}
			if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
				t.Fatalf("Content-Type = %q, want %q", got, "text/event-stream")
			}
		})
	}
}
