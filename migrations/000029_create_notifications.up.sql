CREATE TABLE customer_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    type TEXT NOT NULL,
    related_entity_id UUID,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_customer_notifications_customer_id ON customer_notifications (customer_id, created_at DESC);
CREATE INDEX idx_customer_notifications_unread ON customer_notifications (customer_id) WHERE is_read = false;

CREATE TABLE employee_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    type TEXT NOT NULL,
    related_entity_id UUID,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_employee_notifications_employee_id ON employee_notifications (employee_id, created_at DESC);
CREATE INDEX idx_employee_notifications_unread ON employee_notifications (employee_id) WHERE is_read = false;
