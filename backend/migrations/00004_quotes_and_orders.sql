-- +goose Up
CREATE TABLE quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES users(id),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    configuration JSONB NOT NULL,
    breakdown JSONB NOT NULL,
    fingerprint TEXT NOT NULL CHECK (length(fingerprint) = 64),
    total_cents INTEGER NOT NULL CHECK (total_cents BETWEEN 0 AND 10000000),
    currency TEXT NOT NULL DEFAULT 'EUR' CHECK (currency = 'EUR'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (expires_at > created_at)
);
CREATE INDEX quotes_customer_idx ON quotes(customer_id,created_at);
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES users(id),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    quote_id UUID NOT NULL UNIQUE REFERENCES quotes(id),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 100),
    configuration JSONB NOT NULL,
    breakdown JSONB NOT NULL,
    total_cents INTEGER NOT NULL CHECK (total_cents BETWEEN 0 AND 10000000),
    currency TEXT NOT NULL CHECK (currency = 'EUR'),
    -- A booking is never proof of payment. Payment integration is a separate flow.
    payment_status TEXT NOT NULL DEFAULT 'unpaid' CHECK (payment_status IN ('unpaid','paid','partially_refunded','refunded')),
    status TEXT NOT NULL DEFAULT 'placed' CHECK (status IN ('placed','accepted','received','in_progress','ready','completed','cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(customer_id,idempotency_key)
);
CREATE INDEX orders_customer_idx ON orders(customer_id,created_at DESC,id);
CREATE INDEX orders_stringer_idx ON orders(stringer_id,status,created_at DESC,id);
CREATE TABLE order_status_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    from_status TEXT,
    to_status TEXT NOT NULL CHECK (to_status IN ('placed','accepted','received','in_progress','ready','completed','cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX order_events_order_idx ON order_status_events(order_id,created_at,id);
CREATE TABLE inventory_reservations (
    order_id UUID PRIMARY KEY REFERENCES orders(id),
    lot_id UUID NOT NULL REFERENCES inventory_lots(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    state TEXT NOT NULL DEFAULT 'reserved' CHECK (state IN ('reserved','consumed','released'))
);
CREATE INDEX inventory_reservations_lot_idx ON inventory_reservations(lot_id,state);
ALTER TABLE inventory_movements ADD COLUMN order_id UUID REFERENCES orders(id);

-- +goose Down
ALTER TABLE inventory_movements DROP COLUMN order_id;
DROP TABLE inventory_reservations, order_status_events, orders, quotes;
