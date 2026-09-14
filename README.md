# StatusPulse

StatusPulse is an open-source uptime and performance monitoring control plane. The MVP uses a Go backend, PostgreSQL 16, and a Next.js dashboard.

## MVP architecture

- **Backend:** Go + Gin
- **Database:** PostgreSQL 16
- **Frontend:** Next.js
- **Telemetry:** REST + Server-Sent Events (SSE)
- **Probe engine:** Go worker pool

## Repository structure

```text
statuspulse/
├── backend/
│   ├── cmd/api/              # API entrypoint
│   ├── internal/
│   │   ├── api/              # HTTP handlers
│   │   ├── config/            # configuration loading
│   │   ├── db/                # database connection
│   │   ├── engine/            # scheduler + worker pool
│   │   ├── models/            # domain models
│   │   └── repository/        # PostgreSQL access
│   └── migrations/            # SQL schema migrations
├── frontend/                  # Next.js dashboard
├── docs/                      # architecture / API notes
├── docker-compose.yml
├── .env.example
└── README.md
```

## Start PostgreSQL

```bash
docker compose up -d postgres
```

The initial SQL migration is mounted into PostgreSQL's initialization directory, so a fresh database is created with the MVP tables.

## MVP data model

- `monitors`: target URL and monitoring configuration.
- `check_logs`: raw probe results, retained for the MVP before the later 7-day cleanup job is implemented.
- `incidents`: outage events and recovery timestamps.
- `daily_metrics`: permanent daily rollups; aggregation will be implemented in a later phase.

## Next implementation step

Initialize the Go module and implement the database connection + monitor repository before adding the concurrent probe engine.
