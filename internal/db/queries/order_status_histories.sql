-- name: ListOrderStatusHistoriesByOrder :many
SELECT * FROM order_status_histories
WHERE order_id = $1
ORDER BY created_at ASC;

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

-- name: ListStationHistoryByEmployee :many
SELECT
    h.id, h.order_id, h.old_status, h.new_status, h.created_at,
    o.invoice_number
FROM order_status_histories h
JOIN orders o ON o.id = h.order_id
WHERE h.changed_by_id = $1
  AND h.old_status = $2
  AND h.changed_by_type = 'employee'
ORDER BY h.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountStationHistoryByEmployee :one
SELECT COUNT(*) FROM order_status_histories
WHERE changed_by_id = $1 AND old_status = $2 AND changed_by_type = 'employee';
