-- name: CreateDriverTask :one
INSERT INTO driver_tasks (order_id, task_type, status)
VALUES ($1, $2, 'available')
RETURNING *;

-- name: GetDriverTaskByID :one
SELECT * FROM driver_tasks WHERE id = $1;

-- name: GetActiveDriverTaskByDriver :one
SELECT * FROM driver_tasks WHERE driver_id = $1 AND status = 'in_progress';

-- name: ListAvailableDriverTasksByType :many
SELECT * FROM driver_tasks WHERE task_type = $1 AND status = 'available' ORDER BY created_at ASC;

-- name: ClaimDriverTaskIfAvailable :one
UPDATE driver_tasks
SET driver_id = $1, status = 'in_progress', taken_at = now(), updated_at = now()
WHERE id = $2 AND status = 'available' AND driver_id IS NULL
RETURNING *;

-- name: CompleteDriverTaskIfInProgress :one
UPDATE driver_tasks
SET status = 'completed', completed_at = now(), updated_at = now()
WHERE id = $1 AND status = 'in_progress' AND driver_id = $2
RETURNING *;

-- name: ListDriverTaskHistory :many
SELECT * FROM driver_tasks
WHERE driver_id = $1 AND status = 'completed'
ORDER BY completed_at DESC
LIMIT $2 OFFSET $3;

-- name: CountDriverTaskHistory :one
SELECT count(*) FROM driver_tasks WHERE driver_id = $1 AND status = 'completed';

-- name: DriverPerformanceReport :many
SELECT dt.driver_id AS employee_id, count(*) AS total_jobs
FROM driver_tasks dt
JOIN employees e ON e.id = dt.driver_id
WHERE dt.status = 'completed' AND dt.driver_id IS NOT NULL
  AND (sqlc.narg('outlet_id')::uuid IS NULL OR e.outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR dt.completed_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR dt.completed_at <= sqlc.narg('date_to'))
GROUP BY dt.driver_id;
