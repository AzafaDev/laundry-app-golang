-- name: CreateEmployeeShift :one
INSERT INTO employee_shifts (employee_id, shift_id, outlet_id, day_of_week, date, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetEmployeeShiftByID :one
SELECT * FROM employee_shifts
WHERE id = $1 AND employee_id = $2;

-- name: ListEmployeeShiftsByEmployee :many
SELECT * FROM employee_shifts
WHERE employee_id = $1
ORDER BY created_at DESC;

-- name: DeleteEmployeeShift :exec
DELETE FROM employee_shifts
WHERE id = $1 AND employee_id = $2;

-- name: GetEmployeeShiftByEmployeeAndDate :one
SELECT * FROM employee_shifts
WHERE employee_id = $1 AND date = $2 AND is_active = true;

-- name: GetEmployeeShiftByEmployeeAndDayOfWeek :one
SELECT * FROM employee_shifts
WHERE employee_id = $1 AND day_of_week = $2 AND is_active = true;

-- name: ListEmployeeShiftsForDate :many
SELECT DISTINCT ON (employee_id) *
FROM employee_shifts
WHERE is_active = true AND (date = $1 OR (date IS NULL AND day_of_week = $2))
ORDER BY employee_id, (date IS NOT NULL) DESC;
