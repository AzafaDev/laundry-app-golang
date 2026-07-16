CREATE TABLE order_item_breakdowns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    clothing_type_id UUID NOT NULL REFERENCES clothing_types(id),
    quantity INTEGER NOT NULL,
    created_by UUID NOT NULL REFERENCES employees(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX one_breakdown_per_order_clothing_type ON order_item_breakdowns (order_id, clothing_type_id);
CREATE INDEX idx_order_item_breakdowns_order_id ON order_item_breakdowns (order_id);
