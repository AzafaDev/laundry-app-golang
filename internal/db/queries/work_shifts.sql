-- name: CreateWorkShift :one
INSERT INTO work_shifts (name, start_time, end_time, description, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWorkShiftByID :one
SELECT * FROM work_shifts
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetWorkShiftByIDAny :one
SELECT * FROM work_shifts
WHERE id = $1;

-- name: ListWorkShifts :many
SELECT * FROM work_shifts
WHERE deleted_at IS NULL
ORDER BY name
LIMIT $1 OFFSET $2;

-- name: CountWorkShifts :one
SELECT count(*) FROM work_shifts WHERE deleted_at IS NULL;

-- name: UpdateWorkShift :one
UPDATE work_shifts
SET name = $1, start_time = $2, end_time = $3, description = $4, is_active = $5
WHERE id = $6 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteWorkShift :exec
UPDATE work_shifts
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: HardDeleteWorkShift :exec
DELETE FROM work_shifts
WHERE id = $1;

-- name: CountActiveEmployeeShiftsByShiftID :one
SELECT count(*) FROM employee_shifts
WHERE shift_id = $1 AND is_active = true;
