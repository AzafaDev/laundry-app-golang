-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, laundry_item_id, quantity, price_at_order)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListOrderItemsByOrder :many
SELECT * FROM order_items
WHERE order_id = $1
ORDER BY created_at ASC;

-- name: CountOrderItemsByOrder :one
SELECT count(*) FROM order_items WHERE order_id = $1;
