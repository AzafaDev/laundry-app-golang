-- name: CreateEmailChangeToken :one
INSERT INTO email_change_tokens (customer_id, new_email, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetEmailChangeTokenByHash :one
SELECT * FROM email_change_tokens
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now();

-- name: MarkEmailChangeTokenUsed :exec
UPDATE email_change_tokens
SET used_at = now()
WHERE id = $1;

-- name: DeleteExpiredOrUsedEmailChangeTokens :exec
DELETE FROM email_change_tokens WHERE expires_at < now() OR used_at IS NOT NULL;
