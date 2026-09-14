package engine

import (
	"context"
	"net/http"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func CheckMonitor(ctx context.Context, monitor models.Monitor) models.CheckResult {
	start := time.Now()

	client := &http.Client{
		Timeout: time.Duration(monitor.TimeoutSeconds) * time.Second,
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		monitor.URL,
		nil,
	)
	if err != nil {
		return models.CheckResult{
			MonitorID: monitor.ID,
			CheckedAt: time.Now(),
			Reachable: false,
			Error:     err.Error(),
		}
	}

	response, err := client.Do(request)

	latency := time.Since(start).Milliseconds()

	if err != nil {
		return models.CheckResult{
			MonitorID: monitor.ID,
			CheckedAt: time.Now(),
			LatencyMs: latency,
			Reachable: false,
			Error:     err.Error(),
		}
	}

	defer response.Body.Close()

	return models.CheckResult{
		MonitorID:  monitor.ID,
		CheckedAt:  time.Now(),
		StatusCode: response.StatusCode,
		LatencyMs:  latency,
		Reachable:  response.StatusCode == monitor.ExpectedStatusCode,
	}
}
