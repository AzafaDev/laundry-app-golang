CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_number TEXT NOT NULL,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    pickup_address_id UUID NOT NULL REFERENCES customer_addresses(id),
    status TEXT NOT NULL DEFAULT 'waiting_pickup_driver' CHECK (status IN (
        'waiting_pickup_driver',
        'laundry_to_outlet',
        'laundry_arrived_outlet',
        'washing',
        'ironing',
        'packing',
        'waiting_payment',
        'ready_for_delivery',
        'delivery_to_customer',
        'received_by_customer',
        'completed'
    )),
    pickup_date DATE NOT NULL,
    delivery_fee NUMERIC(10, 2) NOT NULL DEFAULT 0,
    total_price NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_orders_invoice_number ON orders (invoice_number);
CREATE INDEX idx_orders_customer_id ON orders (customer_id);
CREATE INDEX idx_orders_outlet_id ON orders (outlet_id);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_created_at ON orders (created_at);
