CREATE TABLE laundry_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    unit VARCHAR(20) NOT NULL DEFAULT 'pcs' CHECK (unit IN ('pcs', 'kg')),
    base_price NUMERIC(10, 2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX one_active_laundry_item_name ON laundry_items (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_laundry_items_is_active ON laundry_items (is_active);
CREATE INDEX idx_laundry_items_deleted_at ON laundry_items (deleted_at);
