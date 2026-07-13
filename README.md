# laundry-app-with-golang

## Development setup

1. Copy `.env` with the required variables (see `internal/config/config.go` for the full list: `DATABASE_URL`, `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `RESEND_API_KEY`, `CLOUDINARY_URL`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `OPENCAGE_API_KEY`, and optionally `PORT`, `GO_ENV`, `APP_BASE_URL`, `FRONTEND_URL`).
2. Run migrations: `make migrate-up`
3. Seed the first `super_admin` employee: `make seed-admin`
4. Run the API: `make run` (or `make dev` for hot reload via air)

### Seeding the first super_admin

The employee-creation API is RBAC-gated to `super_admin` only, so the very first admin account can't be created through the API. `make seed-admin` (`cmd/seed/main.go`) inserts one directly, hashing the password the same way the API does (`internal/auth.HashPassword`). It's safe to re-run — it's a no-op if an employee with that email already exists.

Override the seeded identity with env vars: `SEED_ADMIN_NAME`, `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD`.

**Default dev credentials** (used when the above env vars aren't set):
- Email: `admin@laundry.test`
- Password: `changeme123`

Dev-only — never rely on these defaults outside a local/dev database.
