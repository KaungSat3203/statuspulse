package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errTestDatabaseNotConfigured = errors.New("TEST_DATABASE_URL is not set")

// Tests never load .env or fall back to the application's DATABASE_URL.
func testDatabaseConfig() (*pgxpool.Config, error) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		return nil, errTestDatabaseNotConfigured
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// Do not include the connection string (which may contain credentials).
		return nil, errors.New("invalid TEST_DATABASE_URL")
	}
	cfg.MaxConns = 4
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	return cfg, nil
}

// Each test gets migrated tables in a private schema. Its search_path deliberately
// excludes public, so even an unqualified DELETE cannot touch application tables.
func newTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := testDatabaseConfig()
	if errors.Is(err, errTestDatabaseNotConfigured) {
		t.Skip("integration test requires an explicit TEST_DATABASE_URL; application DATABASE_URL is never used")
	}
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.ConnectConfig(ctx, cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}

	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("generate test schema name: %v", err)
	}
	// This generated identifier is the only target cleanup is allowed to drop.
	schema := pgx.Identifier{"statuspulse_test_" + hex.EncodeToString(random[:])}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create isolated test schema: %v", err)
	}

	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("remove isolated test schema: %v", err)
		}
		_ = admin.Close(cleanupCtx)
	})

	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("create isolated test pool: %v", err)
	}

	migrations, err := filepath.Glob("../../migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("locate test migrations: %v", err)
	}
	for _, path := range migrations {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		// PostgreSQL 13+ provides gen_random_uuid in pg_catalog. Do not
		// install/drop a database-wide extension inside a disposable schema.
		query := strings.ReplaceAll(string(sql), "CREATE EXTENSION IF NOT EXISTS pgcrypto;", "")
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatalf("apply migration %s to test schema: %v", path, err)
		}
	}
	return pool
}

func TestTestDatabaseConfigNeverUsesApplicationURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://application:secret@localhost/application")
	t.Setenv("TEST_DATABASE_URL", "")
	if _, err := testDatabaseConfig(); !errors.Is(err, errTestDatabaseNotConfigured) {
		t.Fatalf("expected missing test URL, got %v", err)
	}
	t.Setenv("TEST_DATABASE_URL", "postgres://tester@localhost/statuspulse_test?sslmode=disable")
	cfg, err := testDatabaseConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Database != "statuspulse_test" {
		t.Fatalf("unexpected test database: %s", cfg.ConnConfig.Database)
	}
}

func TestTestDatabaseSchemasAreIsolated(t *testing.T) {
	first := newTestDatabase(t)
	second := newTestDatabase(t)
	ctx := context.Background()

	monitor := &models.Monitor{
		Name: "Other schema monitor", URL: "https://example.com",
		ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5,
	}
	if err := NewMonitorRepository(second).Create(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCheckLogRepository(second).CreateBatch(ctx, []models.CheckResult{{
		MonitorID: monitor.ID, CheckedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Reachable: true,
	}}); err != nil {
		t.Fatal(err)
	}
	deleted, err := NewCheckLogRepository(first).DeleteOlderThan(ctx, time.Now())
	if err != nil || deleted != 0 {
		t.Fatalf("isolated cleanup = %d, %v; want 0", deleted, err)
	}
	var count int
	if err := second.QueryRow(ctx, "SELECT COUNT(*) FROM check_logs").Scan(&count); err != nil || count != 1 {
		t.Fatalf("other schema checks = %d, %v; want 1", count, err)
	}
}
