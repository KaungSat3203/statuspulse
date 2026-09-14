package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const (
	defaultIncidentLimit = 20
	maxIncidentLimit     = 100
)

type IncidentReadService interface {
	ListRecent(ctx context.Context, limit int) ([]models.Incident, error)
}

type IncidentHandler struct {
	service IncidentReadService
}

func NewIncidentHandler(service IncidentReadService) *IncidentHandler {
	return &IncidentHandler{service: service}
}

func (h *IncidentHandler) ListIncidents(c *gin.Context) {
	limit, err := parseIncidentLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	incidents, err := h.service.ListRecent(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, incidents)
}

func parseIncidentLimit(value string) (int, error) {
	if value == "" {
		return defaultIncidentLimit, nil
	}

	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if limit < 1 {
		return 1, nil
	}
	if limit > maxIncidentLimit {
		return maxIncidentLimit, nil
	}

	return limit, nil
}
