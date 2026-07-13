-- name: CreateAddress :one
WITH inserted AS (
    INSERT INTO customer_addresses (customer_id, label, address, province_id, city_id, district_id, postal_code, latitude, longitude, is_primary)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    RETURNING *
)
SELECT inserted.*, p.name AS province_name, c.name AS city_name, d.name AS district_name
FROM inserted
JOIN provinces p ON p.id = inserted.province_id
JOIN cities c ON c.id = inserted.city_id
JOIN districts d ON d.id = inserted.district_id;

-- name: ListAddresses :many
SELECT customer_addresses.*, p.name AS province_name, c.name AS city_name, d.name AS district_name
FROM customer_addresses
JOIN provinces p ON p.id = customer_addresses.province_id
JOIN cities c ON c.id = customer_addresses.city_id
JOIN districts d ON d.id = customer_addresses.district_id
WHERE customer_id = $1
ORDER BY is_primary DESC, created_at DESC;

-- name: GetAddressByID :one
SELECT customer_addresses.*, p.name AS province_name, c.name AS city_name, d.name AS district_name
FROM customer_addresses
JOIN provinces p ON p.id = customer_addresses.province_id
JOIN cities c ON c.id = customer_addresses.city_id
JOIN districts d ON d.id = customer_addresses.district_id
WHERE customer_addresses.id = $1 AND customer_addresses.customer_id = $2;

-- name: UpdateAddress :one
WITH updated AS (
    UPDATE customer_addresses
    SET label = $1, address = $2, province_id = $3, city_id = $4, district_id = $5, postal_code = $6, latitude = $7, longitude = $8, updated_at = now()
    WHERE customer_addresses.id = $9 AND customer_addresses.customer_id = $10
    RETURNING *
)
SELECT updated.*, p.name AS province_name, c.name AS city_name, d.name AS district_name
FROM updated
JOIN provinces p ON p.id = updated.province_id
JOIN cities c ON c.id = updated.city_id
JOIN districts d ON d.id = updated.district_id;

-- name: UnsetPrimaryAddresses :exec
UPDATE customer_addresses
SET is_primary = false, updated_at = now()
WHERE customer_id = $1 AND is_primary = true;

-- name: SetAddressPrimary :one
WITH promoted AS (
    UPDATE customer_addresses
    SET is_primary = true, updated_at = now()
    WHERE customer_addresses.id = $1 AND customer_addresses.customer_id = $2
    RETURNING *
)
SELECT promoted.*, p.name AS province_name, c.name AS city_name, d.name AS district_name
FROM promoted
JOIN provinces p ON p.id = promoted.province_id
JOIN cities c ON c.id = promoted.city_id
JOIN districts d ON d.id = promoted.district_id;

-- name: DeleteAddress :exec
DELETE FROM customer_addresses
WHERE id = $1 AND customer_id = $2;

-- name: GetMostRecentAddress :one
SELECT * FROM customer_addresses
WHERE customer_id = $1
ORDER BY created_at DESC
LIMIT 1;
