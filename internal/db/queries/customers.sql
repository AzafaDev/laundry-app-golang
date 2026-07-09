-- name: CreateCustomer :one
INSERT INTO customers (full_name, email, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCustomerByEmail :one
SELECT * FROM customers
WHERE email = $1 AND deleted_at IS NULL;