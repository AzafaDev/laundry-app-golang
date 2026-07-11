-- name: CreateSocialAccount :one
INSERT INTO social_accounts (customer_id, provider, provider_uid)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSocialAccountByProviderAndUID :one
SELECT * FROM social_accounts
WHERE provider = $1 AND provider_uid = $2;
