# StatusPulse MVP Architecture

```text
Next.js Dashboard
       │
       │ REST / SSE
       ▼
Go API + Monitoring Engine
       │
       │ pgx
       ▼
PostgreSQL 16
```

The engine will later use a ticker to schedule active monitors, bounded channels to distribute jobs to a fixed worker pool, and batch writes for probe results.
