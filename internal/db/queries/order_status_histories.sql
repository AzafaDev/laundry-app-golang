-- name: WorkerPerformanceReport :many
-- Approximates "jobs completed by worker" from order_status_histories.
-- Known limitation: a bypass-approved station completion (ReviewBypassRequest)
-- records changed_by_id as the approving admin, not the original worker —
-- this is an accepted gap from deriving performance stats from a generic
-- status-change log instead of a dedicated process_logs table (out of scope).
SELECT osh.changed_by_id AS employee_id, count(*) AS total_jobs
FROM order_status_histories osh
JOIN employees e ON e.id = osh.changed_by_id
WHERE osh.changed_by_type = 'employee'
  AND osh.old_status IN ('washing', 'ironing', 'packing')
  AND (sqlc.narg('outlet_id')::uuid IS NULL OR e.outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR osh.created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR osh.created_at <= sqlc.narg('date_to'))
GROUP BY osh.changed_by_id;
