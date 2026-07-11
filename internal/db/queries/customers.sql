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