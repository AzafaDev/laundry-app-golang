CREATE INDEX idx_order_status_histories_changed_by ON order_status_histories (changed_by_id, created_at DESC);
