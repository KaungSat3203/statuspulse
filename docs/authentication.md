# Authentication, ownership, and deployment

## Request flow

The browser talks to Next.js at `/api/*`; the route handler only proxies known
StatusPulse routes and methods to the fixed `STATUSPULSE_API_URL`. It never
accepts a caller-selected upstream host or an asserted user identity.

Go validates the cookie, attaches the user ID to the request context, and
user-scoped repository SQL applies ownership predicates to monitors and joins
through monitors for checks, metrics, and incidents. Another user's resource
returns 404. Anonymous requests return 401. There is no incident-details route
today; any future detail route must use the same ownership boundary.

Server Components validate `/auth/me`, redirect anonymous users to login, and
use a per-request server client that forwards only the session cookie. Browser
API calls include a CSRF header; tokens are never accessible to JavaScript.

The single dashboard EventSource connects to `/api/events`. Go checks current
ownership and session validity before sending each event, and checks idle
sessions every 15 seconds. New monitors need no new subscription. Deleted or
unowned monitors are filtered out. Revocation/expiry closes streams on their
next event or idle check. In-flight responses/events already authorized cannot
be recalled. Slow streams have bounded writes; monitoring workers remain
independent of HTTP clients.

## Configuration

Backend (`backend/.env` or injected environment):

- `DATABASE_URL`: use the provider's required TLS settings.
- `APP_ENV`: development or production. Production enables Secure cookies and
  requires explicit HTTPS frontend origins.
- `FRONTEND_ORIGINS`: comma-separated exact origins, without paths/trailing
  slashes. Development defaults to `http://localhost:3000`; production has no default.
- `AUTH_RATE_LIMIT_KEY`: base64-encoded random secret containing at least 32
  bytes. Generate locally with `openssl rand -base64 32`. Share across API
  instances; changing it resets effective rate-limit buckets.
- `SESSION_TTL`: default 168h; configurable between 1h and 720h.
- `TRUSTED_PROXY_CIDRS`: empty by default. Only explicitly trusted reverse-proxy
  addresses may supply forwarded client IPs.

Next.js (server-only `frontend/.env.local` or injected environment):

- `STATUSPULSE_API_URL`: Go origin, default `http://localhost:8080`.
- `APP_ENV` and `FRONTEND_ORIGINS`: must match the backend deployment.
- `STATUSPULSE_TRUST_PROXY_IP`: false by default. Set true only if your edge
  proxy strips incoming X-Forwarded-For and sets the authentic client IP.

The former `NEXT_PUBLIC_STATUSPULSE_API_URL` is not used. Do not put database
credentials or authentication secrets in any NEXT_PUBLIC variable.

## Cookies and CSRF

Development uses `statuspulse_session`. Production uses
`__Host-statuspulse_session` with Secure, HttpOnly, Path=/, no Domain, and
SameSite=Lax. Cookie expiration/Max-Age matches the absolute session
expiration. Sessions do not slide automatically.

Login and signup issue a new 256-bit random token and revoke the presented old
session. Only its SHA-256 hash is stored. Logout revokes the presented session
and clears the cookie; it does not sign out every device. If account creation
succeeds but issuing its session fails, retry through login rather than signup.

All mutations—including login/logout—require a trusted Origin and
`X-StatusPulse-CSRF: 1`. JSON endpoints reject non-JSON content. The custom
header is a CSRF defense, not a credential: cross-origin browsers need successful
preflight to set it. Missing/null/untrusted origins are rejected. API CORS only
grants explicitly configured origins with credentials; never a wildcard.

Nonbrowser tools must intentionally supply the configured Origin and CSRF
header for writes. Never change GET endpoints to perform mutations.

## Passwords and rate limiting

Emails are trimmed and lowercased, with a database uniqueness index. Provider
specific rewriting (removing dots or plus suffixes) is not performed.

Passwords accept 15–128 Unicode characters, including spaces, and at most 512
UTF-8 bytes. Argon2id uses a random 16-byte salt, 64 MiB memory, three iterations,
one lane, and a 32-byte output. Parameters are encoded with the hash. At most
two hashes run concurrently per API process; overload returns a safe retryable
error. Benchmark before changing these constants; verification only accepts the
bounded format this version writes.

Invalid email/password logins have the same public error, and missing accounts
still perform a dummy Argon2 verification. Duplicate signup returns a neutral
409 message; signup is not account-enumeration-resistant in the same way as login.

Signup and login each allow at most 30 attempts per client IP and 10 per
normalized email per 15-minute fixed window. The PostgreSQL upsert is atomic;
HMAC-derived keys avoid storing raw IP/email identifiers in the limiter.
Successful attempts consume budget too. This conservative limit also means a
targeted account can be temporarily rate-limited by an attacker.

**Proxy deployment matters:** with the safe defaults, Go sees the Next.js
server's IP, so browser users share that IP budget. For production, configure a
trusted edge that overwrites X-Forwarded-For, enable forwarding in Next.js, trust
only the BFF/edge network in Go, and restrict direct access to the API. Never
enable forwarded-IP trust on an Internet-accessible, unsanitized Next.js server.

## Additive migration and legacy ownership

Migration 004 adds users, sessions, limiter rows, and nullable monitor ownership.
Existing check logs, incidents, daily metrics, and monitor settings remain
unchanged. The engine continues to process unassigned monitors, but they are
not visible through authenticated API reads/SSE. Existing raw-log retention
still runs normally.

Back up the database, apply migration 004 once, and deploy the matching Go and
Next.js versions together. Do not expose a partially updated installation.

An operator must explicitly choose an existing user for each legacy monitor.
Inventory with database read access:

```sql
SELECT id, name, user_id FROM monitors WHERE user_id IS NULL ORDER BY created_at;
SELECT id, email FROM users ORDER BY created_at;
```

From `backend/`, using the intended DATABASE_URL:

```sh
go run ./cmd/assign-monitor -monitor MONITOR_UUID -user USER_UUID
# Review the dry-run result and ownership decision, then explicitly commit:
go run ./cmd/assign-monitor -monitor MONITOR_UUID -user USER_UUID -apply
```

The command checks both records and locks the monitor in a transaction. It
refuses already-owned monitors. It changes only user_id and updated_at; dependent
records and checking configuration remain intact. Keep the explicit mapping in
your operator change record. Do not expose this command through an HTTP route.

After reviewing all assignments, optionally enforce ownership for every row:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f operations/finalize_monitor_ownership.sql
```

This separate operator script refuses to proceed while NULL owners remain.
It is deliberately outside migrations, so a routine migration cannot force
ownership choices. Newly created API monitors always have an authenticated owner.

## Operational considerations and remaining scope

- Terminate HTTPS at a trusted proxy, set APP_ENV=production on both processes,
  protect upstream transport/network access, and keep clocks synchronized.
- Deploy Next.js somewhere that supports long-lived streaming responses;
  disable proxy buffering and allow SSE connection durations. Do not cache
  auth, API, or personalized HTML responses at the CDN.
- Authentication is shared across API instances, but the monitoring scheduler
  and broadcaster are still process-local. Run one monitoring-engine/API
  instance until leader election and shared telemetry are implemented.
- No email verification, password reset, MFA, account deletion, all-device
  logout, or automatic expired-auth-row cleanup is included.
- Periodically clean only expired session/limiter records as a separate
  operational task. These bounded statements can be repeated:
  `DELETE FROM sessions WHERE token_hash IN (SELECT token_hash FROM sessions WHERE expires_at < NOW() LIMIT 1000);`
  `DELETE FROM auth_rate_limits WHERE key_hash IN (SELECT key_hash FROM auth_rate_limits WHERE expires_at < NOW() LIMIT 1000);`
  Revoked sessions remain until their natural expiry.
- Do not log request bodies, Cookie/Set-Cookie, raw URLs containing secrets, or
  database errors with personal data. Go's panic recovery emits only a generic
  message; configure equivalent redaction in your ingress/APM.
- Authentication does not make arbitrary outbound monitor URLs safe. Existing
  probe SSRF/egress hardening remains a separate public-launch consideration.
- PostgreSQL runtime credentials should have only necessary privileges; keep
  schema-migration/operator credentials separate where practical.

## Manual acceptance checks

1. Open / while signed out: it redirects to /login. Signup with a valid email
   and passphrase; verify /login and /signup redirect to the signed-in dashboard.
2. Inspect cookie attributes in DevTools. Confirm no token appears in JSON,
   client storage, or EventSource URLs. Check Secure/__Host behavior under HTTPS.
3. Create/edit/pause/resume a monitor, open history/analytics, refresh, and
   confirm persisted status and live SSE still work.
4. In another browser profile, create a second account. Confirm each profile
   sees only its own monitors/incidents. Requests for the other profile's
   monitor UUID, checks, metrics, PATCH, and DELETE must return 404.
5. Keep SSE open, create/delete monitors, and confirm only current owned checks
   arrive. Logout in another tab: the old stream closes at its next event or
   idle check, without requiring a monitoring-worker restart.
6. Try a mutation with no Origin, an unrelated Origin, or no CSRF header: 403.
   JSON preflight from the allowed origin should permit credentials and the
   required methods/headers. Unrelated origins must receive no CORS permission.
7. Try wrong credentials, malformed input, duplicate email, and repeated login
   attempts. Verify safe feedback, 429/Retry-After, and no token/error leakage.
8. Exercise API outage/recovery and session expiry. The UI should show a safe
   error for outages and redirect after expired authentication—not crash.
9. Verify backfill on a disposable database first. No assignment should happen
   without -apply; finalization must reject remaining unassigned monitors.
