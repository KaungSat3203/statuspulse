package api

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestCheckResultSSEPayloadIsValidJSON(t *testing.T) {
	result := models.CheckResult{
		MonitorID:  "monitor-1",
		StatusCode: 200,
		Reachable:  true,
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal check result: %v", err)
	}

	var event bytes.Buffer
	if err := writeSSEEvent(&event, payload); err != nil {
		t.Fatalf("write SSE event: %v", err)
	}

	var received models.CheckResult
	if err := json.Unmarshal(
		bytes.TrimSuffix(
			bytes.TrimPrefix(event.Bytes(), []byte("event: check\ndata: ")),
			[]byte("\n\n"),
		),
		&received,
	); err != nil {
		t.Fatalf("SSE data payload is not valid JSON: %v", err)
	}

	if received != result {
		t.Fatalf("expected result %+v, got %+v", result, received)
	}
}
