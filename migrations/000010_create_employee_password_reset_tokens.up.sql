CREATE TABLE IF NOT EXISTS employee_password_reset_tokens (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_employee_password_reset_tokens_employee_id ON employee_password_reset_tokens(employee_id);