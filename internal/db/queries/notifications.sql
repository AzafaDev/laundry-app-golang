-- name: CreateCustomerNotification :one
INSERT INTO customer_notifications (customer_id, title, body, type, related_entity_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListCustomerNotifications :many
SELECT * FROM customer_notifications
WHERE customer_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCustomerNotifications :one
SELECT count(*) FROM customer_notifications WHERE customer_id = $1;

-- name: CountUnreadCustomerNotifications :one
SELECT count(*) FROM customer_notifications WHERE customer_id = $1 AND is_read = false;

-- name: MarkCustomerNotificationRead :exec
UPDATE customer_notifications
SET is_read = true
WHERE id = $1 AND customer_id = $2;

-- name: MarkAllCustomerNotificationsRead :exec
UPDATE customer_notifications
SET is_read = true
WHERE customer_id = $1 AND is_read = false;

-- name: CreateEmployeeNotification :one
INSERT INTO employee_notifications (employee_id, title, body, type, related_entity_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEmployeeNotifications :many
SELECT * FROM employee_notifications
WHERE employee_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountEmployeeNotifications :one
SELECT count(*) FROM employee_notifications WHERE employee_id = $1;

-- name: CountUnreadEmployeeNotifications :one
SELECT count(*) FROM employee_notifications WHERE employee_id = $1 AND is_read = false;

-- name: MarkEmployeeNotificationRead :exec
UPDATE employee_notifications
SET is_read = true
WHERE id = $1 AND employee_id = $2;

-- name: MarkAllEmployeeNotificationsRead :exec
UPDATE employee_notifications
SET is_read = true
WHERE employee_id = $1 AND is_read = false;
