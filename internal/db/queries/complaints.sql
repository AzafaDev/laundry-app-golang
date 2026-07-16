-- name: CreateComplaint :one
INSERT INTO complaints (order_id, customer_id, complaint_type, description, photo_urls)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
