-- name: CreateAttendance :one
INSERT INTO attendances (
    employee_id, outlet_id, date,
    check_in_time, check_in_latitude, check_in_longitude,
    is_late, late_minutes, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetAttendanceByEmployeeAndDate :one
SELECT * FROM attendances
WHERE employee_id = $1 AND date = $2;

-- name: CheckOutAttendance :one
UPDATE attendances
SET check_out_time = $1, check_out_latitude = $2, check_out_longitude = $3, updated_at = now()
WHERE employee_id = $4 AND date = $5 AND check_out_time IS NULL
RETURNING *;

-- name: ListAttendancesByEmployee :many
SELECT attendances.*, o.name AS outlet_name
FROM attendances
LEFT JOIN outlets o ON o.id = attendances.outlet_id
WHERE employee_id = $1
ORDER BY date DESC
LIMIT $2 OFFSET $3;

-- name: CountAttendancesByEmployee :one
SELECT count(*) FROM attendances WHERE employee_id = $1;

-- name: ListAttendanceReport :many
SELECT * FROM attendances
WHERE (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('employee_id')::uuid IS NULL OR employee_id = sqlc.narg('employee_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('date_from')::date IS NULL OR date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL OR date <= sqlc.narg('date_to'))
ORDER BY date DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAttendanceReport :one
SELECT count(*) FROM attendances
WHERE (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('employee_id')::uuid IS NULL OR employee_id = sqlc.narg('employee_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('date_from')::date IS NULL OR date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL OR date <= sqlc.narg('date_to'));

-- name: CreateAbsentAttendance :one
INSERT INTO attendances (employee_id, outlet_id, date, status)
VALUES ($1, $2, $3, 'absent')
ON CONFLICT (employee_id, date) DO NOTHING
RETURNING *;

-- name: AutoCheckoutAttendance :exec
UPDATE attendances
SET check_out_time = $1, updated_at = now()
WHERE employee_id = $2 AND date = $3 AND check_in_time IS NOT NULL AND check_out_time IS NULL;
