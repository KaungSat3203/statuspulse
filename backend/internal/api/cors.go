package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const localFrontendOrigin = "http://localhost:3000"

func LocalFrontendCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") == localFrontendOrigin {
			c.Header("Access-Control-Allow-Origin", localFrontendOrigin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}

		c.Next()
	}
}
