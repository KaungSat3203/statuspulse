package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/KaungSat3203/statuspulse/backend/internal/service"
)

type MonitorHandler struct {
	service *service.MonitorService
}

func NewMonitorHandler(service *service.MonitorService) *MonitorHandler {
	return &MonitorHandler{service: service}
}

type CreateMonitorRequest struct {
	Name               string `json:"name"`
	URL                string `json:"url"`
	ExpectedStatusCode int    `json:"expected_status_code"`
	IntervalSeconds    int    `json:"interval_seconds"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
}

type UpdateMonitorRequest struct {
	Name               *string `json:"name"`
	URL                *string `json:"url"`
	ExpectedStatusCode *int    `json:"expected_status_code"`
	IntervalSeconds    *int    `json:"interval_seconds"`
	TimeoutSeconds     *int    `json:"timeout_seconds"`
	Active             *bool   `json:"active"`
}

func (h *MonitorHandler) CreateMonitor(c *gin.Context) {
	var request CreateMonitorRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	monitor := &models.Monitor{
		Name:               request.Name,
		URL:                request.URL,
		ExpectedStatusCode: request.ExpectedStatusCode,
		IntervalSeconds:    request.IntervalSeconds,
		TimeoutSeconds:     request.TimeoutSeconds,
	}
	if err := h.service.CreateMonitor(c.Request.Context(), monitor); err != nil {
		monitorError(c, err)
		return
	}
	c.JSON(http.StatusCreated, monitor)
}

func (h *MonitorHandler) ListMonitors(c *gin.Context) {
	monitors, err := h.service.ListMonitors(c.Request.Context())
	if err != nil {
		monitorError(c, err)
		return
	}
	c.JSON(http.StatusOK, monitors)
}

func (h *MonitorHandler) GetMonitor(c *gin.Context) {
	if !isValidMonitorID(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitor ID"})
		return
	}
	monitor, err := h.service.GetMonitor(c.Request.Context(), c.Param("id"))
	if err != nil {
		monitorError(c, err)
		return
	}
	c.JSON(http.StatusOK, monitor)
}

func (h *MonitorHandler) UpdateMonitor(c *gin.Context) {
	id := c.Param("id")
	if !isValidMonitorID(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitor ID"})
		return
	}

	var request UpdateMonitorRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	monitor, err := h.service.UpdateMonitor(c.Request.Context(), id, models.MonitorUpdate{
		Name:               request.Name,
		URL:                request.URL,
		ExpectedStatusCode: request.ExpectedStatusCode,
		IntervalSeconds:    request.IntervalSeconds,
		TimeoutSeconds:     request.TimeoutSeconds,
		Active:             request.Active,
	})
	if err != nil {
		monitorError(c, err)
		return
	}
	c.JSON(http.StatusOK, monitor)
}

func (h *MonitorHandler) DeleteMonitor(c *gin.Context) {
	id := c.Param("id")
	if !isValidMonitorID(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitor ID"})
		return
	}

	if err := h.service.DeleteMonitor(c.Request.Context(), id); err != nil {
		monitorError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func isValidMonitorID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, character := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') && !(character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}
