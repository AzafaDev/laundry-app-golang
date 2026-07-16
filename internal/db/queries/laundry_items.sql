-- name: CreateLaundryItem :one
INSERT INTO laundry_items (name, description, unit, base_price, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetLaundryItemByID :one
SELECT * FROM laundry_items
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetLaundryItemByIDAny :one
SELECT * FROM laundry_items
WHERE id = $1;

-- name: ListLaundryItems :many
SELECT * FROM laundry_items
WHERE deleted_at IS NULL
ORDER BY name
LIMIT $1 OFFSET $2;

-- name: CountLaundryItems :one
SELECT count(*) FROM laundry_items WHERE deleted_at IS NULL;

-- name: ListActiveLaundryItems :many
SELECT * FROM laundry_items
WHERE is_active = true AND deleted_at IS NULL
ORDER BY name;

-- name: UpdateLaundryItem :one
UPDATE laundry_items
SET name = $1, description = $2, unit = $3, base_price = $4, is_active = $5, updated_at = now()
WHERE id = $6 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteLaundryItem :exec
UPDATE laundry_items
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: HardDeleteLaundryItem :exec
DELETE FROM laundry_items
WHERE id = $1;
