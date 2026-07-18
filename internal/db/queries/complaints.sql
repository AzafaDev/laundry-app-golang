-- name: CreateComplaint :one
INSERT INTO complaints (order_id, customer_id, complaint_type, description, photo_urls)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListComplaints :many
SELECT c.* FROM complaints c
JOIN orders o ON o.id = c.order_id
WHERE (sqlc.narg('outlet_id')::uuid IS NULL OR o.outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR o.invoice_number ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY c.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListComplaintsByOrder :many
SELECT * FROM complaints
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: CountComplaints :one
SELECT count(*) FROM complaints c
JOIN orders o ON o.id = c.order_id
WHERE (sqlc.narg('outlet_id')::uuid IS NULL OR o.outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR o.invoice_number ILIKE '%' || sqlc.narg('search') || '%');

-- name: CountComplaintsByStatus :many
SELECT c.status, count(*) AS total FROM complaints c
JOIN orders o ON o.id = c.order_id
WHERE sqlc.narg('outlet_id')::uuid IS NULL OR o.outlet_id = sqlc.narg('outlet_id')
GROUP BY c.status;

-- name: GetComplaintByID :one
SELECT * FROM complaints WHERE id = $1;

-- name: UpdateComplaintStatus :one
UPDATE complaints
SET status = $1,
    resolution_notes = $2,
    expected_resolution_date = $3,
    resolved_by = $4,
    resolved_at = $5,
    updated_at = now()
WHERE id = $6
RETURNING *;

-- name: ListComplaintsForAdmin :many
SELECT c.id, c.order_id, c.customer_id, c.complaint_type, c.description, c.photo_urls, c.status,
       c.expected_resolution_date, c.resolved_by, c.resolution_notes, c.resolved_at, c.created_at, c.updated_at,
       o.invoice_number, cu.full_name AS customer_name, cu.phone AS customer_phone,
       e.full_name AS resolved_by_name
FROM complaints c
JOIN orders o ON o.id = c.order_id
JOIN customers cu ON cu.id = c.customer_id
LEFT JOIN employees e ON e.id = c.resolved_by
WHERE (sqlc.narg('outlet_id')::uuid IS NULL OR o.outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR o.invoice_number ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY c.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetComplaintByIDForAdmin :one
SELECT c.id, c.order_id, c.customer_id, c.complaint_type, c.description, c.photo_urls, c.status,
       c.expected_resolution_date, c.resolved_by, c.resolution_notes, c.resolved_at, c.created_at, c.updated_at,
       o.invoice_number, cu.full_name AS customer_name, cu.phone AS customer_phone,
       e.full_name AS resolved_by_name
FROM complaints c
JOIN orders o ON o.id = c.order_id
JOIN customers cu ON cu.id = c.customer_id
LEFT JOIN employees e ON e.id = c.resolved_by
WHERE c.id = $1;
