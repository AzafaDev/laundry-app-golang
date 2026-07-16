CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    amount NUMERIC(12, 2) NOT NULL,
    payment_method TEXT NOT NULL DEFAULT 'gateway',
    gateway_name TEXT,
    gateway_transaction_id TEXT,
    gateway_response JSONB,
    payment_link TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'expired')),
    expired_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_gateway_transaction_id ON payments (gateway_transaction_id);
CREATE INDEX idx_payments_status ON payments (status);
CREATE INDEX idx_payments_expired_at ON payments (expired_at);
