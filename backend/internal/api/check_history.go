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

type CheckHistoryReadService interface {
	ListCheckHistory(ctx context.Context, monitorID string, limit int) ([]models.CheckResult, error)
}

type CheckHistoryHandler struct {
	service CheckHistoryReadService
}

func NewCheckHistoryHandler(service CheckHistoryReadService) *CheckHistoryHandler {
	return &CheckHistoryHandler{service: service}
}

func (h *CheckHistoryHandler) ListChecks(c *gin.Context) {
	monitorID := c.Param("id")
	if !isValidMonitorID(monitorID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitor ID"})
		return
	}

	limit, err := parseCheckHistoryLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := h.service.ListCheckHistory(c.Request.Context(), monitorID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "monitor not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load check history"})
		return
	}
	c.JSON(http.StatusOK, results)
}

func parseCheckHistoryLimit(value string) (int, error) {
	if value == "" {
		return service.DefaultCheckHistoryLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if limit < 1 {
		return 0, fmt.Errorf("limit must be at least 1")
	}
	return limit, nil
}
