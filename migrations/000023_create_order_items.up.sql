CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    laundry_item_id UUID NOT NULL REFERENCES laundry_items(id),
    quantity NUMERIC(6, 2) NOT NULL,
    price_at_order NUMERIC(10, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);
CREATE INDEX idx_order_items_laundry_item_id ON order_items (laundry_item_id);
