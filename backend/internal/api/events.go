package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/KaungSat3203/statuspulse/backend/internal/telemetry"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type EventMonitorReader interface {
	GetMonitor(context.Context, string) (*models.Monitor, error)
}
type EventHandler struct {
	broadcaster *telemetry.Broadcaster
	auth        *AuthHandler
	monitors    EventMonitorReader
	heartbeat   time.Duration
}

func NewEventHandler(b *telemetry.Broadcaster, a *AuthHandler, m EventMonitorReader) *EventHandler {
	return &EventHandler{broadcaster: b, auth: a, monitors: m, heartbeat: 15 * time.Second}
}

func (h *EventHandler) Stream(c *gin.Context) {
	if _, err := auth.UserID(c.Request.Context()); err != nil {
		authError(c, err)
		return
	}
	subscriber, unsubscribe := h.broadcaster.Subscribe()
	defer unsubscribe()
	timer := time.NewTicker(h.heartbeat)
	defer timer.Stop()
	token, _ := c.Cookie(h.auth.settings.CookieName())

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	controller := http.NewResponseController(c.Writer)
	// Bound slow-client writes as well as database authorization work.
	flush := func() error {
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return controller.Flush()
	}
	c.Status(http.StatusOK)
	if err := flush(); err != nil {
		return
	}
	for {
		var payload []byte
		select {
		case <-c.Request.Context().Done():
			return
		case <-timer.C:
		case next, ok := <-subscriber:
			if !ok {
				return
			}
			payload = next
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		session, err := h.auth.service.Authenticate(ctx, token)
		if err != nil {
			cancel()
			if errors.Is(err, auth.ErrUnauthenticated) {
				_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_, _ = io.WriteString(c.Writer, "event: session-ended\ndata: {}\n\n")
				_ = flush()
			}
			return
		}
		if payload != nil {
			var check models.CheckResult
			if json.Unmarshal(payload, &check) != nil {
				cancel()
				continue
			}
			_, err = h.monitors.GetMonitor(auth.WithUser(ctx, session.User.ID), check.MonitorID)
			cancel()
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return
			} // Ownership lookup errors fail closed.
			_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := writeSSEEvent(c.Writer, payload); err != nil {
				return
			}
		} else {
			cancel()
			_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := io.WriteString(c.Writer, ": keep-alive\n\n"); err != nil {
				return
			}
		}
		if err := flush(); err != nil {
			return
		}
	}
}

func writeSSEEvent(w io.Writer, payload []byte) error {
	_, err := fmt.Fprintf(w, "event: check\ndata: %s\n\n", payload)
	return err
}
