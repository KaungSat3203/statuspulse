package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/KaungSat3203/statuspulse/backend/internal/telemetry"
	"github.com/gin-gonic/gin"
)

type expiringAuth struct {
	fakeAuth
	expired atomic.Bool
}

func (s *expiringAuth) Authenticate(ctx context.Context, token string) (*models.Session, error) {
	if s.expired.Load() {
		return nil, auth.ErrUnauthenticated
	}
	return s.fakeAuth.Authenticate(ctx, token)
}
func TestIdleSSERevalidatesSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := &expiringAuth{}
	h := NewAuthHandler(sessions, auth.Settings{})
	stream := NewEventHandler(telemetry.NewBroadcaster(), h, nil)
	stream.heartbeat = 5 * time.Millisecond
	router := gin.New()
	router.GET("/events", h.Authenticate(), stream.Stream)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events", nil)
	req.AddCookie(&http.Cookie{Name: h.settings.CookieName(), Value: "opaque-cookie"})
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	sessions.expired.Store(true)
	scanner := bufio.NewScanner(response.Body)
	found := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "event: session-ended") {
			found = true
		}
	}
	if !found || scanner.Err() != nil {
		t.Fatalf("idle session not closed cleanly: %v", scanner.Err())
	}
}

func TestSSECancellationReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAuthHandler(&fakeAuth{}, auth.Settings{})
	stream := NewEventHandler(telemetry.NewBroadcaster(), h, nil)
	ended := make(chan struct{})
	router := gin.New()
	router.GET("/events", h.Authenticate(), func(c *gin.Context) { defer close(ended); stream.Stream(c) })
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events", nil)
	req.AddCookie(&http.Cookie{Name: h.settings.CookieName(), Value: "opaque-cookie"})
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	cancel()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("disconnected SSE handler did not return")
	}
}
