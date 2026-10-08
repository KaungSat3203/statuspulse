# StatusPulse

StatusPulse is an uptime monitoring application built with Go/Gin, PostgreSQL
(Neon or local PostgreSQL), Next.js, and Tailwind CSS.

The dashboard supports user-owned monitors, create/edit/delete, pause/resume,
live SSE checks, check history, incidents, and daily metrics/analytics. Background
workers check active monitors, manage incidents, aggregate daily metrics, and
retain seven calendar days of raw checks.

## Architecture

- Go owns authentication, server-side sessions, authorization, and monitoring.
- PostgreSQL stores users, hashed session tokens, monitors, checks, incidents,
  daily metrics, and shared authentication rate limits.
- Next.js Server Components forward the session cookie directly to Go for reads.
- Browser requests use the allowlisted same-origin Next.js `/api/*` proxy.
  It forwards HttpOnly cookies and streams SSE without adding another connection.
- API services use user-scoped repositories. Background jobs keep their global
  repositories and do not need user sessions.

## Local setup

1. Start PostgreSQL with `docker compose up -d postgres`, or use a dedicated Neon
   development database. Fresh Docker databases apply migrations automatically;
   existing databases need the new migration applied explicitly.
2. Copy `.env.example` to `backend/.env`. Fill `DATABASE_URL`, and generate a
   private `AUTH_RATE_LIMIT_KEY` using `openssl rand -base64 32`. Keep both local.
3. For an existing installation with migrations 001–003 already applied:
   from `backend/`, load your database URL in the shell and run
   `psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -1 -f migrations/004_auth.sql`.
   Do not reapply an already-applied migration. Back up the database first.
4. Copy `frontend/.env.example` to `frontend/.env.local`.
5. Run `go run ./cmd/api` from `backend/`.
6. Run `npm install`, then `npm run dev`, from `frontend/`.
7. Open `http://localhost:3000/signup`. New accounts see only their own monitors.

Use `localhost` consistently, not a mixture of localhost and 127.0.0.1.
`APP_ENV=development` uses HTTP cookies; use `APP_ENV=production` on both
processes for a real HTTPS deployment. `next build` alone does not select the
cookie environment.

**Existing monitors are preserved but hidden until explicitly assigned.**
They continue running in the engine. Nothing assigns them to the first signup.
See [Authentication and deployment](docs/authentication.md) for the dry-run
backfill command and optional ownership finalization.

## Validation

```sh
cd backend
go test ./...
# Use an explicit dedicated TEST_DATABASE_URL for database integration tests.

cd ../frontend
node --test tests/*.test.mjs
npm run lint
npm run build
```

See [backend test isolation](backend/TESTING.md). Never use an application database
as an integration-test database.

## Authentication and security

Signup/login/logout/me, Argon2id passwords, hashed opaque sessions, strict
Origin/custom-header CSRF protection, credentialed origin allowlisting, and
repository-level user ownership are implemented.

Read [the authentication guide](docs/authentication.md) before deployment. It
covers environment variables, cookies, rate limits and trusted proxies, legacy
ownership, manual two-user checks, and remaining production limitations.
