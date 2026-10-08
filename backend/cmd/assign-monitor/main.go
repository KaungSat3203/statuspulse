// assign-monitor assigns exactly one explicitly selected legacy monitor.
// Default: validate and roll back. Mutations require -apply.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/db"
	"github.com/KaungSat3203/statuspulse/backend/internal/repository"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		// run has returned, so its deferred pool/transaction cleanup has run.
		os.Exit(1)
	}
}

func run() error {
	monitorID := flag.String("monitor", "", "legacy monitor UUID")
	userID := flag.String("user", "", "existing user UUID")
	apply := flag.Bool("apply", false, "commit this explicit ownership assignment")
	flag.Parse()
	if *monitorID == "" || *userID == "" {
		return errors.New("both -monitor and -user are required; default is a dry run")
	}
	_ = godotenv.Load()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := db.Connect(url)
	if err != nil {
		return errors.New("database connection failed; verify configuration")
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := repository.NewMonitorRepository(pool).AssignLegacyMonitor(ctx, *monitorID, *userID, *apply); err != nil {
		return errors.New("assignment failed; verify both UUIDs exist, the monitor is unowned, and database access is available")
	}
	if *apply {
		fmt.Printf("Assigned monitor %s to user %s.\n", *monitorID, *userID)
	} else {
		fmt.Printf("Dry run passed: monitor %s -> user %s. No data changed; use -apply to commit.\n", *monitorID, *userID)
	}
	return nil
}
