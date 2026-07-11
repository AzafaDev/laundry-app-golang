-- name: CreateCustomer :one
INSERT INTO customers (full_name, email, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCustomerByEmail :one
SELECT * FROM customers
WHERE email = $1 AND deleted_at IS NULL;

-- name: VerifyCustomerEmail :exec
UPDATE customers
SET is_verified = true
WHERE id = $1;

-- name: UpdateCustomerPassword :one
UPDATE customers
SET password_hash = $1
WHERE id = $2
RETURNING *;

-- name: GetCustomerByID :one
SELECT * FROM customers
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateCustomerProfile :one
UPDATE customers
SET full_name = $1, phone = $2
WHERE id = $3
RETURNING *;

-- name: UpdateCustomerEmail :one
UPDATE customers
SET email = $1
WHERE id = $2
RETURNING *;

-- name: UpdateCustomerAvatar :one
UPDATE customers
SET avatar_url = $1
WHERE id = $2
RETURNING *;

-- name: IncrementCustomerTokenVersion :one
UPDATE customers
SET token_version = token_version + 1
WHERE id = $1
RETURNING *;