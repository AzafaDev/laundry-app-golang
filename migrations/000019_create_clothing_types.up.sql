CREATE TABLE clothing_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX one_active_clothing_type_name ON clothing_types (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_clothing_types_is_active ON clothing_types (is_active);
CREATE INDEX idx_clothing_types_deleted_at ON clothing_types (deleted_at);
