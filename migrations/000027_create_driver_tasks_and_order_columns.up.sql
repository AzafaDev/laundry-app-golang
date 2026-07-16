ALTER TABLE orders
  ADD COLUMN pickup_schedule TIMESTAMPTZ,
  ADD COLUMN auto_confirm_at TIMESTAMPTZ;

CREATE TABLE driver_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    driver_id UUID REFERENCES employees(id),
    task_type TEXT NOT NULL CHECK (task_type IN ('pickup', 'delivery')),
    status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'in_progress', 'completed')),
    taken_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX one_task_per_order_and_type ON driver_tasks (order_id, task_type);
CREATE INDEX idx_driver_tasks_order_id ON driver_tasks (order_id);
CREATE INDEX idx_driver_tasks_driver_id ON driver_tasks (driver_id);
CREATE INDEX idx_driver_tasks_status ON driver_tasks (status);
CREATE INDEX idx_driver_tasks_task_type ON driver_tasks (task_type);
