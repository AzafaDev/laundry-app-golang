-- name: CreateOrderItemBreakdown :one
INSERT INTO order_item_breakdowns (order_id, clothing_type_id, quantity, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListOrderItemBreakdownsByOrder :many
SELECT * FROM order_item_breakdowns
WHERE order_id = $1
ORDER BY created_at ASC;
