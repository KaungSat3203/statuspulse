package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	router *gin.Engine,
	monitorHandler *MonitorHandler,
	checkHistoryHandler *CheckHistoryHandler,
	incidentHandler *IncidentHandler,
	eventHandler *EventHandler,
	dailyMetricsHistoryHandler *DailyMetricsHistoryHandler,
) {
	router.Use(LocalFrontendCORS())

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
