-- name: CreateEmployeeRefreshToken :one
INSERT INTO employee_refresh_tokens (employee_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetEmployeeRefreshTokenByHash :one
SELECT * FROM employee_refresh_tokens
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeEmployeeRefreshToken :exec
UPDATE employee_refresh_tokens
SET revoked_at = now()
WHERE id = $1;

-- name: RevokeEmployeeRefreshTokensByEmployeeID :exec
UPDATE employee_refresh_tokens
SET revoked_at = now()
WHERE employee_id = $1;

-- name: DeleteExpiredOrRevokedEmployeeRefreshTokens :exec
DELETE FROM employee_refresh_tokens WHERE expires_at < now() OR revoked_at IS NOT NULL;
