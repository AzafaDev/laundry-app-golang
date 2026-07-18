-- name: CreateBypassRequest :one
INSERT INTO bypass_requests (
    order_id, station, requested_by, expected_items, actual_items,
    discrepancy_description, photo_evidence, attempt_number
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPendingBypassRequest :one
SELECT * FROM bypass_requests
WHERE order_id = $1 AND station = $2 AND status = 'pending';

-- name: CountNonPendingBypassRequests :one
SELECT count(*) FROM bypass_requests
WHERE order_id = $1 AND station = $2 AND status != 'pending';

-- name: GetBypassRequestByID :one
SELECT * FROM bypass_requests
WHERE id = $1;

-- name: ListBypassRequestsByOrder :many
SELECT * FROM bypass_requests
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: ListBypassRequests :many
SELECT br.* FROM bypass_requests br
JOIN orders o ON o.id = br.order_id
WHERE (sqlc.narg('outlet_id')::uuid IS NULL OR o.outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('status')::text IS NULL OR br.status = sqlc.narg('status'))
ORDER BY br.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountBypassRequests :one
SELECT count(*) FROM bypass_requests br
JOIN orders o ON o.id = br.order_id
WHERE (sqlc.narg('outlet_id')::uuid IS NULL OR o.outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('status')::text IS NULL OR br.status = sqlc.narg('status'));

-- name: ReviewBypassRequest :one
UPDATE bypass_requests
SET status = $1, reviewed_by = $2, admin_notes = $3, resolved_at = now()
WHERE id = $4 AND status = 'pending'
RETURNING *;

-- name: GetLatestBypassStatusByOrder :one
SELECT status FROM bypass_requests
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1;
