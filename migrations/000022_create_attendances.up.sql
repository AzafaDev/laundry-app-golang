CREATE TABLE attendances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    date DATE NOT NULL,
    check_in_time TIMESTAMPTZ,
    check_in_latitude NUMERIC(10, 8),
    check_in_longitude NUMERIC(11, 8),
    check_out_time TIMESTAMPTZ,
    check_out_latitude NUMERIC(10, 8),
    check_out_longitude NUMERIC(11, 8),
    notes TEXT,
    is_late BOOLEAN,
    late_minutes INTEGER,
    status VARCHAR(20) CHECK (status IN ('on_time', 'late', 'absent')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX one_attendance_per_employee_per_day ON attendances (employee_id, date);
CREATE INDEX idx_attendances_outlet_id ON attendances (outlet_id);
CREATE INDEX idx_attendances_date ON attendances (date);
CREATE INDEX idx_attendances_status ON attendances (status);
