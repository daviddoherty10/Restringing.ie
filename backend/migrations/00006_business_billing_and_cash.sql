-- +goose Up
CREATE TABLE business_subscriptions (
    stringer_id UUID PRIMARY KEY REFERENCES stringers(id),
    customer_id TEXT UNIQUE,
    subscription_id TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'none',
    paid_until TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE booking_usage (
    order_id UUID PRIMARY KEY REFERENCES orders(id),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    accepted_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX booking_usage_month_idx ON booking_usage(stringer_id,accepted_at);
-- Include existing accepted orders, including later cancellations. Never reset
-- usage simply because a business changes plans or an order changes status.
INSERT INTO booking_usage(order_id,stringer_id,accepted_at)
SELECT o.id,o.stringer_id,COALESCE(min(e.created_at),o.created_at)
FROM orders o LEFT JOIN order_status_events e ON e.order_id=o.id AND e.to_status='accepted'
GROUP BY o.id
HAVING min(e.created_at) IS NOT NULL OR o.status IN ('accepted','received','in_progress','ready','completed');
CREATE TABLE billing_checkout_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    price_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    session_id TEXT UNIQUE,
    checkout_url TEXT,
    resolved BOOLEAN NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX billing_checkout_open_idx ON billing_checkout_attempts(stringer_id) WHERE NOT resolved;
CREATE TABLE billing_events (
    event_id TEXT PRIMARY KEY,
    stringer_id UUID REFERENCES stringers(id),
    event_type TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE orders ADD COLUMN payment_method TEXT NOT NULL DEFAULT 'cash' CHECK(payment_method IN ('cash','online'));
CREATE TABLE order_payment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL CHECK(to_status IN ('unpaid','paid')),
    reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX order_payment_events_order_idx ON order_payment_events(order_id,created_at,id);

-- +goose Down
DROP TABLE order_payment_events;
ALTER TABLE orders DROP COLUMN payment_method;
DROP TABLE billing_events,billing_checkout_attempts,booking_usage,business_subscriptions;
