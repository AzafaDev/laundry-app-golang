ALTER TABLE employees ADD COLUMN outlet_id UUID REFERENCES outlets(id);
