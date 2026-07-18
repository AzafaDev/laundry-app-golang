-- name: CreateOrder :one
INSERT INTO orders (invoice_number, customer_id, outlet_id, pickup_address_id, status, pickup_date, delivery_fee, total_price)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders
WHERE id = $1 AND customer_id = $2;

-- name: ListOrders :many
SELECT orders.*, o.name AS outlet_name, o.address AS outlet_address
FROM orders
LEFT JOIN outlets o ON o.id = orders.outlet_id
WHERE orders.customer_id = sqlc.arg('customer_id')
  AND (sqlc.narg('status')::text IS NULL OR orders.status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR orders.invoice_number ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR orders.created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR orders.created_at <= sqlc.narg('date_to'))
ORDER BY orders.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetOrderByIDWithOutlet :one
SELECT orders.*, o.name AS outlet_name, o.address AS outlet_address
FROM orders
LEFT JOIN outlets o ON o.id = orders.outlet_id
WHERE orders.id = $1 AND orders.customer_id = $2;

-- name: CountOrders :one
SELECT count(*) FROM orders
WHERE customer_id = sqlc.arg('customer_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR invoice_number ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR created_at <= sqlc.narg('date_to'));

-- name: ListOrdersByOutlet :many
SELECT orders.*, o.name AS outlet_name, o.address AS outlet_address,
       c.full_name AS customer_name, c.phone AS customer_phone
FROM orders
LEFT JOIN outlets o ON o.id = orders.outlet_id
LEFT JOIN customers c ON c.id = orders.customer_id
WHERE orders.outlet_id = sqlc.arg('outlet_id')
  AND (sqlc.narg('status')::text IS NULL OR orders.status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR orders.invoice_number ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR orders.created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR orders.created_at <= sqlc.narg('date_to'))
ORDER BY orders.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountOrdersByOutlet :one
SELECT count(*) FROM orders
WHERE outlet_id = sqlc.arg('outlet_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR invoice_number ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR created_at <= sqlc.narg('date_to'));

-- name: CountActiveOrdersByCustomer :one
SELECT COUNT(*) FROM orders
WHERE customer_id = $1 AND status != 'completed';

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

-- name: ListOrdersReadyForAutoComplete :many
SELECT * FROM orders
WHERE status = 'received_by_customer' AND auto_confirm_at IS NOT NULL AND auto_confirm_at <= now();

-- name: ClaimOrderForTask :one
-- No status guard here — replicates the TS source's runClaimTransaction,
-- which sets the order's status unconditionally once the driver_tasks
-- optimistic-concurrency claim itself has already succeeded. $2 is NULL for
-- delivery claims; COALESCE preserves the pickup_schedule set earlier by
-- the pickup claim instead of clobbering it back to NULL.
UPDATE orders
SET status = $1, pickup_schedule = COALESCE($2, pickup_schedule), updated_at = now()
WHERE id = $3
RETURNING *;

-- name: CompleteOrderForTaskIfCurrent :one
-- $2 is NULL for pickup completion; COALESCE avoids clobbering
-- auto_confirm_at back to NULL on a later (delivery) completion that
-- reuses this same query with a non-NULL value.
UPDATE orders
SET status = $1, auto_confirm_at = COALESCE($2, auto_confirm_at), updated_at = now()
WHERE id = $3 AND status = $4
RETURNING *;

-- name: SalesReportByPeriod :many
SELECT date_trunc(sqlc.arg('group_by')::text, updated_at)::timestamptz AS period,
       COALESCE(SUM(total_price), 0)::numeric AS income,
       count(*) AS order_count
FROM orders
WHERE status = 'completed'
  AND (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR updated_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR updated_at <= sqlc.narg('date_to'))
GROUP BY period
ORDER BY period;

-- name: SalesReportSummary :one
SELECT COALESCE(SUM(total_price), 0)::numeric AS total_income, count(*) AS total_orders
FROM orders
WHERE status = 'completed'
  AND (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR updated_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR updated_at <= sqlc.narg('date_to'));
