package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	router *gin.Engine,
	monitorHandler *MonitorHandler,
	checkHistoryHandler *CheckHistoryHandler,
	incidentHandler *IncidentHandler,
	eventHandler *EventHandler,
	dailyMetricsHistoryHandler *DailyMetricsHistoryHandler,
	authHandler *AuthHandler,
) {
	router.Use(authHandler.BrowserSecurity())
	router.POST("/auth/signup", authHandler.Signup)
	router.POST("/auth/login", authHandler.Login)
	// Logout is idempotent even after expiry, but is always CSRF-protected.
	router.POST("/auth/logout", authHandler.Logout)
	router.Use(authHandler.Authenticate())
	router.GET("/auth/me", authHandler.Me)

	router.POST("/monitors", monitorHandler.CreateMonitor)
	router.GET("/monitors", monitorHandler.ListMonitors)
	router.GET("/monitors/:id/checks", checkHistoryHandler.ListChecks)
	router.GET("/monitors/:id/metrics", dailyMetricsHistoryHandler.ListMetrics)
	router.GET("/monitors/:id", monitorHandler.GetMonitor)
	router.PATCH("/monitors/:id", monitorHandler.UpdateMonitor)
	router.GET("/incidents", incidentHandler.ListIncidents)
	router.DELETE("/monitors/:id", monitorHandler.DeleteMonitor)
	router.GET("/events", eventHandler.Stream)
}
