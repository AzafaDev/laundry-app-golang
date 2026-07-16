-- name: CreateOutlet :one
INSERT INTO outlets (name, address, latitude, longitude, is_active, service_radius_km)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOutletByID :one
SELECT * FROM outlets
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListOutlets :many
SELECT * FROM outlets
WHERE deleted_at IS NULL
ORDER BY name
LIMIT $1 OFFSET $2;

-- name: CountOutlets :one
SELECT count(*) FROM outlets WHERE deleted_at IS NULL;

-- name: UpdateOutlet :one
UPDATE outlets
SET name = $1, address = $2, latitude = $3, longitude = $4, is_active = $5, service_radius_km = $6, updated_at = now()
WHERE id = $7 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteOutlet :exec
UPDATE outlets
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListActiveOutlets :many
SELECT * FROM outlets
WHERE is_active = true AND deleted_at IS NULL;
