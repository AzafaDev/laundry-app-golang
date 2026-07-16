-- name: CreateOrder :one
INSERT INTO orders (invoice_number, customer_id, outlet_id, pickup_address_id, status, pickup_date, delivery_fee, total_price)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders
WHERE id = $1 AND customer_id = $2;

-- name: ListOrders :many
SELECT * FROM orders
WHERE customer_id = sqlc.arg('customer_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR invoice_number ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR created_at <= sqlc.narg('date_to'))
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountOrders :one
SELECT count(*) FROM orders
WHERE customer_id = sqlc.arg('customer_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR invoice_number ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR created_at <= sqlc.narg('date_to'));

-- name: CreateOrderStatusHistory :one
INSERT INTO order_status_histories (order_id, old_status, new_status, changed_by_type, changed_by_id, note)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOrderByIDAny :one
SELECT * FROM orders
WHERE id = $1;

-- name: UpdateOrderStatusIfCurrent :one
UPDATE orders
SET status = $1, updated_at = now()
WHERE id = $2 AND status = $3
RETURNING *;

-- name: ProcessOrderIfCurrent :one
UPDATE orders
SET status = $1, total_price = $2, total_weight_kg = $3, updated_at = now()
WHERE id = $4 AND status = $5
RETURNING *;

-- name: ListOrdersByOutletAndStatus :many
SELECT * FROM orders
WHERE outlet_id = $1 AND status = $2
ORDER BY created_at ASC;
