package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/KaungSat3203/statuspulse/backend/internal/api"
	"github.com/KaungSat3203/statuspulse/backend/internal/config"
	"github.com/KaungSat3203/statuspulse/backend/internal/db"
	"github.com/KaungSat3203/statuspulse/backend/internal/engine"
	"github.com/KaungSat3203/statuspulse/backend/internal/repository"
	"github.com/KaungSat3203/statuspulse/backend/internal/service"
	"github.com/KaungSat3203/statuspulse/backend/internal/telemetry"
)

const shutdownTimeout = 10 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("load configuration: %v", err)
		return
	}

	pool, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Printf("connect to database: %v", err)
		return
	}
	defer pool.Close()

	monitorRepository := repository.NewMonitorRepository(pool)
	checkLogRepository := repository.NewCheckLogRepository(pool)
	incidentRepository := repository.NewIncidentRepository(pool)
	dailyMetricsRepository := repository.NewDailyMetricsRepository(pool)
	broadcaster := telemetry.NewBroadcaster()

	monitorService := service.NewMonitorService(monitorRepository)
	checkHistoryService := service.NewCheckHistoryService(monitorRepository, checkLogRepository)
	incidentService := service.NewIncidentService(monitorRepository, incidentRepository)
	dailyMetricsService := service.NewDailyMetricsService(dailyMetricsRepository)

	monitorHandler := api.NewMonitorHandler(monitorService)
	checkHistoryHandler := api.NewCheckHistoryHandler(checkHistoryService)
	incidentHandler := api.NewIncidentHandler(incidentService)
	eventHandler := api.NewEventHandler(broadcaster)
	dailyMetricsHistoryService := service.NewDailyMetricsHistoryService(monitorRepository, dailyMetricsRepository)
	dailyMetricsHistoryHandler := api.NewDailyMetricsHistoryHandler(dailyMetricsHistoryService)

	dailyMetricsJob := engine.NewDailyMetricsJob(monitorRepository, dailyMetricsService)
	checkLogRetentionJob := engine.NewCheckLogRetentionJob(checkLogRepository)

	runner := engine.NewRunner(monitorRepository, checkLogRepository, incidentService, broadcaster)

	router := gin.Default()
	api.RegisterRoutes(router, monitorHandler, checkHistoryHandler, incidentHandler, eventHandler, dailyMetricsHistoryHandler)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runnerDone := make(chan struct{})
	go func() {
		defer close(runnerDone)
		runner.Start(ctx)
	}()

	dailyMetricsDone := make(chan struct{})
	go func() {
		defer close(dailyMetricsDone)
		dailyMetricsJob.Run(ctx)
	}()

	checkLogRetentionDone := make(chan struct{})
	go func() {
		defer close(checkLogRetentionDone)
		checkLogRetentionJob.Run(ctx)
	}()

	server := &http.Server{Addr: ":8080", Handler: router}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()

	log.Println("StatusPulse API starting...")
	log.Println("Connected to Neon PostgreSQL")
	log.Println("HTTP server listening on :8080")

	select {
	case <-ctx.Done():
		log.Println("Shutdown signal received")
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped unexpectedly: %v", err)
		}
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("graceful HTTP shutdown: %v", err)
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			log.Printf("force close HTTP server: %v", closeErr)
		}
	}

	<-runnerDone
	<-dailyMetricsDone
	<-checkLogRetentionDone
}
