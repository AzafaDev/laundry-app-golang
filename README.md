# Laundry Management API (Go)

A production-style backend for a multi-outlet laundry service — order pipeline, driver dispatch, payment integration, attendance tracking, and admin reporting — built in Go as a full port of an existing Node/TypeScript service, with several deliberate architecture improvements along the way.

Frontend: [laundry-app-typescript-react](https://github.com/AzafaDev/laundry-app-typescript-react) (React + TypeScript)

## Why this project exists

This started as a 1:1 port of a working Node.js/Express/Prisma backend, then grew into an exercise in doing the same domain *better* in Go: replacing ad-hoc concurrency handling with proper optimistic-concurrency SQL patterns, swapping Socket.IO for a lighter SSE implementation, and closing real security gaps (CSRF, rate limiting, structured error responses) that existed in the original.

Every non-trivial change was verified two ways before being considered done: automated tests, and live testing against a running server with real HTTP requests — several genuine race conditions (driver task double-claiming, payment webhook idempotency, a worker/payment station race) were only caught this way, not by reading the code.

## Tech stack

| | |
|---|---|
| Language | Go 1.26 |
| HTTP | [Gin](https://github.com/gin-gonic/gin) |
| Database | PostgreSQL via [pgx/v5](https://github.com/jackc/pgx) (no ORM) |
| Query generation | [sqlc](https://sqlc.dev/) — raw SQL in, typed Go out |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Auth | JWT (HS256) in httpOnly cookies, bcrypt password hashing |
| Real-time | Server-Sent Events (custom in-memory pub/sub) |
| Payments | Midtrans (Snap + Core API + webhook) |
| Email | Resend |
| File storage | Cloudinary |
| Geocoding | OpenCage |

## Architecture highlights

- **Optimistic concurrency, not locks.** Every state transition that can race (order status changes, driver task claims, payment confirmation) is guarded by a SQL `UPDATE ... WHERE status = $expected` pattern — the loser of a race gets zero rows affected and a clean 409, instead of a lock or a corrupted state. See `internal/order/handler_driver.go` (`ClaimDriverTaskIfAvailable`) and `internal/payment/handler_helpers.go` (`applyPaymentStatus`).
- **SSE over WebSocket.** The original used Socket.IO for real-time updates, but every event in this system is server→client only — a full WebSocket library would have added complexity nothing here needs. `internal/sse` implements a plain channel-based pub/sub with the same "rooms" semantics (`user:<id>`, `role:<role>`, `outlet:<id>`) over stdlib `http.Flusher`.
- **CSRF via double-submit cookie.** Session cookies use `SameSite=None` in production (the frontend is a separate origin), which disables the browser's built-in CSRF protection. `internal/csrf` closes that gap: a non-httpOnly `csrf_token` cookie the frontend echoes back in an `X-CSRF-Token` header, checked with a constant-time compare.
- **Rate limiting.** `internal/ratelimit` — a per-IP token bucket (`golang.org/x/time/rate`), three tiers: a generous global baseline, a strict login limiter that only counts *failed* attempts against the budget, and a mid-strictness limiter for other auth endpoints (register, password reset, etc.).
- **Structured errors, not leaked internals.** Every handler responds with `{"error": "<code>"}` on failure; raw Go errors are logged server-side only, never forwarded to the client (`internal/apperr`).
- **Timezone correctness.** Attendance/shift logic runs entirely in `Asia/Jakarta` civil time via a single `shift.CivilDateStart` helper — replacing a subtle bug class where `time.Truncate(24*time.Hour)` silently operates on absolute duration since the Unix epoch, not local wall-clock date.

## Domain coverage

- **Orders** — creation with outlet-coverage/delivery-fee calculation, full processing pipeline (admin intake → wash → iron → pack → payment → delivery), item-quantity mismatch detection with bypass-approval workflow, complaints.
- **Driver workflow** — pickup/delivery task claiming and completion, race-safe.
- **Payments** — Midtrans Snap transaction creation, webhook handling with idempotency and amount cross-checking.
- **Attendance & shifts** — geofenced check-in/out, shift scheduling, automatic absence sweep.
- **Admin** — master data (laundry items, clothing types, outlets), employee management, sales/attendance/employee-performance reports (JSON + CSV export).
- **Notifications** — in-app, delivered in real time over SSE.
- **Cron** — auto-completion of orders after a confirmation window, expired-token cleanup — runnable on a schedule or triggered manually by an admin.

## Live demo

- Frontend: https://app.laundry-app-api.my.id
- API: https://laundry-app-api.my.id (health check: `/health`)

Demo accounts (all share password `demo123`) — see [Getting started](#getting-started) below.

## Getting started

**Prerequisites:** Go 1.26+, PostgreSQL, [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI, [`air`](https://github.com/air-verse/air) (optional, for hot reload).

```bash
# 1. Start a local Postgres (or point DATABASE_URL at any Postgres instance)
docker compose up -d

# 2. Configure environment
cp .env.example .env
# edit .env — see below for what each variable needs

# 3. Run migrations
make migrate-up

# 4. Seed a super_admin account to log in with
make seed-admin

# 4b. (Optional) Seed a fuller demo dataset — multiple outlets, one employee
#     per role, sample customers, and orders spread across every pipeline
#     stage, so there's something to look at immediately
make seed-demo
# Every seeded account (all employee roles + all customers) shares the
# password `demo123`, e.g.:
#   admin@demo.laundry              (super_admin)
#   outlet.admin@demo.laundry       (outlet_admin, Laundry Kilat - Curug)
#   washing@demo.laundry            (washing_worker, Curug, shift pagi)
#   washing.sore@demo.laundry       (washing_worker, Curug, shift sore)
#   ironing@demo.laundry            (ironing_worker, Curug)
#   packing@demo.laundry            (packing_worker, Curug)
#   driver@demo.laundry             (driver, Curug)
#   (same role set again with a ".bsd" suffix for the second outlet,
#    Laundry Kilat - BSD, e.g. outlet.admin.bsd@demo.laundry)
#   rina@demo.customer, clean@demo.customer
# Safe to re-run — it's idempotent and won't create duplicates.

# 5. Run the API
make run       # or `make dev` for hot reload via air
```

The server starts on `:8080` (configurable via `PORT`).

### Environment variables

See [`.env.example`](.env.example) for the full list. A few notes:

- `DATABASE_URL`, `JWT_ACCESS_SECRET`, `JWT_EMPLOYEE_ACCESS_SECRET` are the only ones that matter for local development against core features.
- `RESEND_API_KEY`, `CLOUDINARY_URL`, `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`, `OPENCAGE_API_KEY`, `MIDTRANS_SERVER_KEY`/`MIDTRANS_CLIENT_KEY` are all required by `config.Load()` (it fails fast if any are missing) but only matter functionally if you're exercising email, file upload, Google OAuth, geocoding, or payment flows respectively — any non-empty placeholder value is enough to boot the server for everything else.

## Testing

```bash
go test ./...            # full suite
go test ./... -race      # required for internal/order and internal/payment,
                          # which specifically test concurrent request handling
```

Tests run as real integration tests against a live Postgres database (not mocks) — each test creates and tears down its own fixtures (unique emails, dedicated rows), so the suite is safe to run repeatedly against a shared database. Coverage spans:

- Pure-logic unit tests (timezone handling, item-mismatch detection, pagination, rate-limit middleware).
- Full HTTP-layer integration tests for auth, the order pipeline, payments, cron jobs, attendance, notifications, reports, and SSE.
- Concurrency regression tests for the three race conditions mentioned above — each runs multiple goroutines against the same resource and asserts exactly one winner, run with Go's race detector.

## Project layout

```
cmd/
  api/          entrypoint — wires config, calls internal/app
  seed/         one-off super_admin bootstrap script
  seed-demo/    idempotent portfolio demo dataset (outlets, all roles, orders across every pipeline stage)
internal/
  app/          wires every handler/client together (shared by main and tests)
  server/       route table
  <domain>/     one package per business domain (order, payment, attendance, ...),
                each with its own handlers, DTOs, and _test.go files
  db/generated/ sqlc-generated query code (do not edit by hand)
  testutil/     shared test fixtures and helpers
migrations/     golang-migrate SQL migrations, applied in order
```

## Deployment

Single VPS (2 vCPU/2GB), no containers:

```
Internet → HTTPS (Caddy, auto TLS via Let's Encrypt) → Go binary (systemd, localhost:8080) → PostgreSQL (localhost:5432)
```

- **Caddy** terminates TLS and reverse-proxies to the Go binary; only Caddy is exposed to the internet — the API and Postgres are bound to localhost.
- **systemd** keeps the API running and restarts it on failure.
- Deploys are manual: `git pull && go build -o bin/server ./cmd/api && sudo systemctl restart laundry-api`.
- No CI/CD yet — every deploy is a manual SSH + rebuild.

## Known limitations

- Rate limiting and SSE pub/sub are in-memory and per-instance — horizontal scaling would need a shared backend (Redis) for both to remain correct across multiple server instances.
- No load/performance testing has been done — the test suite verifies correctness, not throughput.
- CSRF protection requires the frontend to read the `csrf_token` cookie and echo it back in an `X-CSRF-Token` header on every mutating request; deploying this API without a frontend that does so will reject those requests.
