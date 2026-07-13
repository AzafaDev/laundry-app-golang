-- name: GetEmployeeByID :one
SELECT * FROM employees
WHERE id = $1 AND deleted_at IS NULL;
