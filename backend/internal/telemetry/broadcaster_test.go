package telemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

func TestBroadcaster_SubscribeAndUnsubscribe(t *testing.T) {
	broadcaster := NewBroadcaster()

	subscriber, unsubscribe := broadcaster.Subscribe()
	if broadcaster.SubscriberCount() != 1 {
		t.Fatalf(
			"expected one subscriber, got %d",
			broadcaster.SubscriberCount(),
		)
	}

	unsubscribe()
	unsubscribe()

	if broadcaster.SubscriberCount() != 0 {
		t.Fatalf(
			"expected no subscribers, got %d",
			broadcaster.SubscriberCount(),
		)
	}

	if _, ok := <-subscriber; ok {
		t.Fatal("expected subscriber channel to be closed")
	}
}

func TestBroadcaster_PublishToMultipleSubscribers(t *testing.T) {
	broadcaster := NewBroadcaster()
	first, firstUnsubscribe := broadcaster.Subscribe()
	defer firstUnsubscribe()
	second, secondUnsubscribe := broadcaster.Subscribe()
	defer secondUnsubscribe()

	result := models.CheckResult{
		MonitorID:  "monitor-1",
		StatusCode: 200,
		Reachable:  true,
	}

	broadcaster.Publish(result)

	for _, subscriber := range []<-chan []byte{first, second} {
		payload := receivePayload(t, subscriber)

		var received models.CheckResult
		if err := json.Unmarshal(payload, &received); err != nil {
			t.Fatalf("unmarshal published result: %v", err)
		}
		if received != result {
			t.Fatalf("expected result %+v, got %+v", result, received)
		}
	}
}

func TestBroadcaster_DisconnectedSubscriberIsRemoved(t *testing.T) {
	broadcaster := NewBroadcaster()
	_, unsubscribe := broadcaster.Subscribe()

	unsubscribe()
	broadcaster.Publish(models.CheckResult{MonitorID: "monitor-1"})

	if broadcaster.SubscriberCount() != 0 {
		t.Fatalf(
			"expected disconnected subscriber to be removed, got %d",
			broadcaster.SubscriberCount(),
		)
	}
}

func TestBroadcaster_SlowSubscriberDoesNotBlockPublisher(t *testing.T) {
	broadcaster := NewBroadcasterWithBuffer(1)
	slowSubscriber, unsubscribe := broadcaster.Subscribe()
	defer unsubscribe()

	broadcaster.Publish(models.CheckResult{MonitorID: "first"})

	published := make(chan struct{})
	go func() {
		broadcaster.Publish(models.CheckResult{MonitorID: "second"})
		close(published)
	}()

	select {
	case <-published:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("publishing blocked on a slow subscriber")
	}

	payload := receivePayload(t, slowSubscriber)

	var received models.CheckResult
	if err := json.Unmarshal(payload, &received); err != nil {
		t.Fatalf("unmarshal published result: %v", err)
	}
	if received.MonitorID != "second" {
		t.Fatalf("expected latest result, got %q", received.MonitorID)
	}
}

func TestBroadcaster_PublishesJSONPayload(t *testing.T) {
	broadcaster := NewBroadcaster()
	subscriber, unsubscribe := broadcaster.Subscribe()
	defer unsubscribe()

	checkedAt := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.UTC)
	result := models.CheckResult{
		MonitorID:  "monitor-1",
		CheckedAt:  checkedAt,
		StatusCode: 503,
		LatencyMs:  42,
		Reachable:  false,
		Error:      "unexpected status code",
	}

	broadcaster.Publish(result)
	payload := receivePayload(t, subscriber)

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("published payload is not JSON: %v", err)
	}

	if body["monitor_id"] != result.MonitorID {
		t.Fatalf(
			"expected monitor_id %q, got %v",
			result.MonitorID,
			body["monitor_id"],
		)
	}
	if body["reachable"] != result.Reachable {
		t.Fatalf(
			"expected reachable %v, got %v",
			result.Reachable,
			body["reachable"],
		)
	}
}

func receivePayload(t *testing.T, subscriber <-chan []byte) []byte {
	t.Helper()

	select {
	case payload, ok := <-subscriber:
		if !ok {
			t.Fatal("subscriber channel closed before receiving a payload")
		}
		return payload
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a telemetry payload")
		return nil
	}
}
