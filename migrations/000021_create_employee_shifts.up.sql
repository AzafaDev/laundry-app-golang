CREATE TABLE employee_shifts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    shift_id UUID NOT NULL REFERENCES work_shifts(id),
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    day_of_week SMALLINT CHECK (day_of_week BETWEEN 0 AND 6),
    date DATE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((day_of_week IS NOT NULL) != (date IS NOT NULL))
);

CREATE UNIQUE INDEX one_shift_per_employee_day_of_week ON employee_shifts (employee_id, day_of_week) WHERE day_of_week IS NOT NULL;
CREATE UNIQUE INDEX one_shift_per_employee_date ON employee_shifts (employee_id, date) WHERE date IS NOT NULL;
CREATE INDEX idx_employee_shifts_outlet_day ON employee_shifts (outlet_id, day_of_week);
CREATE INDEX idx_employee_shifts_outlet_date ON employee_shifts (outlet_id, date);
CREATE INDEX idx_employee_shifts_shift_id ON employee_shifts (shift_id);
CREATE INDEX idx_employee_shifts_employee_id ON employee_shifts (employee_id);
