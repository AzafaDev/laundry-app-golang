-- name: CreateEmployee :one
INSERT INTO employees (full_name, email, phone, password_hash, role, is_active, outlet_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateEmployeeOutlet :one
UPDATE employees
SET outlet_id = $1
WHERE id = $2
RETURNING *;

-- name: ListEmployees :many
SELECT employees.*, o.name AS outlet_name, o.deleted_at AS outlet_deleted_at
FROM employees
LEFT JOIN outlets o ON o.id = employees.outlet_id
WHERE (employees.deleted_at IS NULL OR sqlc.arg(include_deleted)::boolean)
  AND (sqlc.narg(role)::text IS NULL OR employees.role = sqlc.narg(role))
  AND (sqlc.narg(search)::text IS NULL OR employees.full_name ILIKE '%' || sqlc.narg(search) || '%' OR employees.email ILIKE '%' || sqlc.narg(search) || '%')
ORDER BY employees.created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountEmployees :one
SELECT count(*) FROM employees
WHERE (deleted_at IS NULL OR sqlc.arg(include_deleted)::boolean)
  AND (sqlc.narg(role)::text IS NULL OR role = sqlc.narg(role))
  AND (sqlc.narg(search)::text IS NULL OR full_name ILIKE '%' || sqlc.narg(search) || '%' OR email ILIKE '%' || sqlc.narg(search) || '%');

-- name: GetEmployeeByIDAny :one
SELECT * FROM employees WHERE id = $1;

-- name: UpdateEmployee :one
UPDATE employees
SET full_name = $1, phone = $2, role = $3, updated_at = now()
WHERE id = $4 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteEmployee :exec
UPDATE employees SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: HardDeleteEmployee :exec
DELETE FROM employees WHERE id = $1;

-- name: GetEmployeeByID :one
SELECT * FROM employees
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEmployeeByEmail :one
SELECT * FROM employees
WHERE email = $1 AND deleted_at IS NULL;

-- name: IncrementEmployeeTokenVersion :one
UPDATE employees
SET token_version = token_version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateEmployeePassword :one
-- Unconditionally reactivates the employee (is_active = TRUE) alongside the
-- password change. Safe today because is_active=false only ever means
-- "never activated." If/when an admin-deactivate feature ships, that
-- feature MUST invalidate this employee's outstanding
-- employee_password_reset_tokens at deactivation time — otherwise a
-- deactivated employee can self-reactivate via ForgotPassword, which is
-- intentionally generic and does not check is_active.
UPDATE employees
SET password_hash = $1, is_active = TRUE
WHERE id = $2
RETURNING *;
