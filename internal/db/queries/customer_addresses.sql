-- name: CreateAddress :one
INSERT INTO customer_addresses (customer_id, label, address, province, city, district, postal_code, latitude, longitude)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListAddresses :many
SELECT * FROM customer_addresses
WHERE customer_id = $1
ORDER BY is_primary DESC, created_at DESC;

-- name: GetAddressByID :one
SELECT * FROM customer_addresses
WHERE id = $1 AND customer_id = $2;

-- name: UpdateAddress :one
UPDATE customer_addresses
SET label = $1, address = $2, province = $3, city = $4, district = $5, postal_code = $6, latitude = $7, longitude = $8, updated_at = now()
WHERE id = $9 AND customer_id = $10
RETURNING *;

-- name: UnsetPrimaryAddresses :exec
UPDATE customer_addresses
SET is_primary = false, updated_at = now()
WHERE customer_id = $1 AND is_primary = true;

-- name: SetAddressPrimary :one
UPDATE customer_addresses
SET is_primary = true, updated_at = now()
WHERE id = $1 AND customer_id = $2
RETURNING *;

-- name: DeleteAddress :exec
DELETE FROM customer_addresses
WHERE id = $1 AND customer_id = $2;

-- name: GetMostRecentAddress :one
SELECT * FROM customer_addresses
WHERE customer_id = $1
ORDER BY created_at DESC
LIMIT 1;
