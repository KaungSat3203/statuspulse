package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/KaungSat3203/statuspulse/backend/internal/service"
)

type DailyMetricsReadService interface {
	ListMonitorMetrics(ctx context.Context, monitorID string, limit int) ([]models.DailyMetric, error)
}

type DailyMetricsHistoryHandler struct {
	service DailyMetricsReadService
}

func NewDailyMetricsHistoryHandler(service DailyMetricsReadService) *DailyMetricsHistoryHandler {
	return &DailyMetricsHistoryHandler{service: service}
}

func (h *DailyMetricsHistoryHandler) ListMetrics(c *gin.Context) {
	monitorID := c.Param("id")
	if !isValidMonitorID(monitorID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitor ID"})
		return
	}

	limit, err := parseDailyMetricsLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metrics, err := h.service.ListMonitorMetrics(c.Request.Context(), monitorID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "monitor not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func parseDailyMetricsLimit(value string) (int, error) {
	if value == "" {
		return service.DefaultDailyMetricsLimit, nil
	}

	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if limit < 1 {
		return 0, fmt.Errorf("limit must be at least 1")
	}
	if limit > service.MaxDailyMetricsLimit {
		return service.MaxDailyMetricsLimit, nil
	}

	return limit, nil
}
