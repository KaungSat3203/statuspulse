# Backend tests

Run unit tests without a database:

```sh
cd backend
go test ./...
```

Repository integration tests skip with an explicit message unless
`TEST_DATABASE_URL` is set. They never load an application `.env` file and
never fall back to `DATABASE_URL`.

For the full suite, supply a dedicated PostgreSQL test database (local PostgreSQL
or a separate Neon test database/branch):

```sh
cd backend
TEST_DATABASE_URL='postgres://test_user:test_password@localhost:5432/statuspulse_test?sslmode=disable' go test -count=1 ./...
```

Use the provider's required TLS settings for remote databases. Keep credentials
out of version control. CI must supply `TEST_DATABASE_URL` to execute repository
integration tests; a unit-only run is not a substitute.

## Isolation

Every integration test creates a randomly named `statuspulse_test_...` schema,
applies the repository migrations, and uses a connection pool with only that
schema in its search path. `public` is excluded. This prevents the retention
test's table-wide deletion from affecting another test or application tables.
Connections use UTC regardless of the database's default timezone.

The database user needs permission to create schemas in the dedicated database.
Tests use PostgreSQL 13+ (the project's PostgreSQL 16 setup is supported).
The helper omits the initial migration's `CREATE EXTENSION pgcrypto` statement:
`gen_random_uuid()` is built into supported PostgreSQL versions, and extensions
must not be installed into disposable test schemas.

Cleanup closes the test pool and drops only the randomly generated schema.
An interrupted test process may leave such a schema behind; inspect its exact
name before removing it. Application tables and database configuration are never
part of cleanup.

## Persistence regression coverage

The suite checks that:

- A deleted monitor cannot invalidate another monitor's batch results.
- Concurrent deletion and batch insertion coordinate through row locks.
- A failed COPY rolls back the entire batch and returns no committed results.
- Cancellation releases database operations without publishing false failures.
- The runner processes only confirmed committed results.
- Deleted monitors are removed from future schedules.
- A persistence error is returned without advancing incidents or SSE.

The runner intentionally does not automatically replay failed/uncertain commits.
The next scheduled cycle performs fresh checks. Durable retry and replay would
require a stable check identity to prevent duplicate logs and failure counters.
