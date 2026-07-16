-- name: CreateClothingType :one
INSERT INTO clothing_types (name, is_active)
VALUES ($1, $2)
RETURNING *;

-- name: GetClothingTypeByID :one
SELECT * FROM clothing_types
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListClothingTypes :many
SELECT * FROM clothing_types
WHERE deleted_at IS NULL
ORDER BY name
LIMIT $1 OFFSET $2;

-- name: CountClothingTypes :one
SELECT count(*) FROM clothing_types WHERE deleted_at IS NULL;

-- name: UpdateClothingType :one
UPDATE clothing_types
SET name = $1, is_active = $2, updated_at = now()
WHERE id = $3 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteClothingType :exec
UPDATE clothing_types
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
