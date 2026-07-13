-- name: CreateEmployeePasswordResetToken :one
INSERT INTO employee_password_reset_tokens (employee_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetEmployeePasswordResetTokenByHash :one
SELECT * FROM employee_password_reset_tokens
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now();

-- name: MarkEmployeePasswordResetTokenUsed :exec
UPDATE employee_password_reset_tokens
SET used_at = now()
WHERE id = $1;
