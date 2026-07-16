CREATE TABLE bypass_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    station TEXT NOT NULL CHECK (station IN ('washing', 'ironing', 'packing')),
    requested_by UUID NOT NULL REFERENCES employees(id),
    expected_items JSONB NOT NULL,
    actual_items JSONB NOT NULL,
    discrepancy_description TEXT NOT NULL,
    photo_evidence TEXT[] NOT NULL DEFAULT '{}',
    attempt_number INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID REFERENCES employees(id),
    admin_notes TEXT,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bypass_requests_order_id ON bypass_requests (order_id);
CREATE INDEX idx_bypass_requests_status ON bypass_requests (status);
CREATE INDEX idx_bypass_requests_reviewed_by ON bypass_requests (reviewed_by);
CREATE INDEX idx_bypass_requests_station ON bypass_requests (station);
