package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestCheckMonitor_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	monitor := models.Monitor{
		ID:                 "test-monitor",
		URL:                server.URL,
		ExpectedStatusCode: 200,
		TimeoutSeconds:     5,
	}

	result := CheckMonitor(context.Background(), monitor)

	if !result.Reachable {
		t.Error("expected monitor to be reachable")
	}

	if result.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", result.StatusCode)
	}

	if result.LatencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", result.LatencyMs)
	}

	if result.Error != "" {
		t.Errorf("expected no error, got %q", result.Error)
	}
}

func TestCheckMonitor_WrongStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	monitor := models.Monitor{
		ID:                 "test-monitor",
		URL:                server.URL,
		ExpectedStatusCode: 200,
		TimeoutSeconds:     5,
	}

	result := CheckMonitor(context.Background(), monitor)

	if result.Reachable {
		t.Error("expected monitor to be unreachable")
	}

	if result.StatusCode != 500 {
		t.Errorf("expected status code 500, got %d", result.StatusCode)
	}

	if result.Error != "" {
		t.Errorf("expected no network error, got %q", result.Error)
	}
}

func TestCheckMonitor_Unreachable(t *testing.T) {
	monitor := models.Monitor{
		ID:                 "test-monitor",
		URL:                "http://127.0.0.1:1",
		ExpectedStatusCode: 200,
		TimeoutSeconds:     1,
	}

	result := CheckMonitor(context.Background(), monitor)

	if result.Reachable {
		t.Error("expected monitor to be unreachable")
	}

	if result.Error == "" {
		t.Error("expected an error for unreachable monitor")
	}
}

func TestCheckMonitor_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	monitor := models.Monitor{
		ID:                 "test-monitor",
		URL:                server.URL,
		ExpectedStatusCode: 200,
		TimeoutSeconds:     1,
	}

	result := CheckMonitor(context.Background(), monitor)

	if result.Reachable {
		t.Error("expected monitor to be unreachable after timeout")
	}

	if result.Error == "" {
		t.Error("expected timeout error")
	}

	if !strings.Contains(strings.ToLower(result.Error), "timeout") {
		t.Errorf("expected timeout error, got %q", result.Error)
	}
}
