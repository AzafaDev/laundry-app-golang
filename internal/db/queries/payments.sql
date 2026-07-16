-- name: UpsertPaymentForOrder :one
-- Replicates the TS source's behavior as-is: a second create-transaction
-- request for the same order overwrites the row with a fresh Midtrans
-- transaction, orphaning the previous payment_link — not fixed here.
INSERT INTO payments (order_id, amount, gateway_name, gateway_transaction_id, gateway_response, payment_link, status, expired_at)
VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
ON CONFLICT (order_id) DO UPDATE SET
    amount = EXCLUDED.amount,
    gateway_name = EXCLUDED.gateway_name,
    gateway_transaction_id = EXCLUDED.gateway_transaction_id,
    gateway_response = EXCLUDED.gateway_response,
    payment_link = EXCLUDED.payment_link,
    status = 'pending',
    expired_at = EXCLUDED.expired_at,
    updated_at = now()
RETURNING *;

-- name: GetPaymentByOrderID :one
SELECT * FROM payments WHERE order_id = $1;

-- name: GetPaymentByGatewayTransactionID :one
SELECT * FROM payments WHERE gateway_transaction_id = $1;

-- name: UpdatePaymentStatus :one
UPDATE payments
SET status = $1, gateway_response = $2, paid_at = $3, updated_at = now()
WHERE id = $4
RETURNING *;
