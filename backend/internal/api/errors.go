package api

import (
	"errors"
	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"net/http"
)

func monitorError(c *gin.Context, err error) {
	var invalid *service.ValidationError
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		authError(c, err)
	case errors.Is(err, pgx.ErrNoRows):
		c.JSON(http.StatusNotFound, gin.H{"error": "monitor not found"})
	case errors.As(err, &invalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": invalid.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to process monitor request"})
	}
}
