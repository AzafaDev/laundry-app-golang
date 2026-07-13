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
