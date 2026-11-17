-- +goose Up
CREATE TABLE stringer_payment_settings (
 stringer_id UUID PRIMARY KEY REFERENCES stringers(id),
 onsite_cash BOOLEAN NOT NULL DEFAULT true,
 onsite_card BOOLEAN NOT NULL DEFAULT false,
 online_enabled BOOLEAN NOT NULL DEFAULT false,
 account_id TEXT UNIQUE,
 account_live BOOLEAN,
 account_ready BOOLEAN NOT NULL DEFAULT false,
 CHECK(onsite_cash OR onsite_card OR online_enabled)
);
ALTER TABLE orders ADD COLUMN payment_review_required BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE orders DROP CONSTRAINT orders_payment_method_check;
ALTER TABLE orders ADD CONSTRAINT orders_payment_method_check CHECK(payment_method IN ('cash','onsite','online'));
ALTER TABLE order_payment_events ALTER COLUMN actor_id DROP NOT NULL;
ALTER TABLE order_payment_events DROP CONSTRAINT order_payment_events_to_status_check;
ALTER TABLE order_payment_events ADD CONSTRAINT order_payment_events_to_status_check CHECK(to_status IN ('unpaid','paid','partially_refunded','refunded'));
CREATE TABLE order_checkout_attempts (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 order_id UUID NOT NULL REFERENCES orders(id),
 account_id TEXT NOT NULL,
 session_id TEXT UNIQUE,
 payment_intent TEXT,
 expires_at TIMESTAMPTZ NOT NULL,
 resolved BOOLEAN NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX one_open_order_checkout ON order_checkout_attempts(order_id) WHERE NOT resolved;
CREATE TABLE racket_payment_events(event_id TEXT PRIMARY KEY, order_id UUID NOT NULL REFERENCES orders(id), event_type TEXT NOT NULL, account_id TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());

-- +goose Down
-- Refuse a rollback that would mislabel historical payment records.
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM orders WHERE payment_method='onsite') OR EXISTS(SELECT 1 FROM order_checkout_attempts) OR EXISTS(SELECT 1 FROM racket_payment_events) OR EXISTS(SELECT 1 FROM stringer_payment_settings WHERE account_id IS NOT NULL) THEN
 RAISE EXCEPTION 'Payment history exists; retain this migration and roll forward instead';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE orders DROP COLUMN payment_review_required;
DROP TABLE racket_payment_events;
DROP TABLE order_checkout_attempts;
ALTER TABLE order_payment_events DROP CONSTRAINT order_payment_events_to_status_check;
ALTER TABLE order_payment_events ADD CONSTRAINT order_payment_events_to_status_check CHECK(to_status IN ('unpaid','paid'));
ALTER TABLE order_payment_events ALTER COLUMN actor_id SET NOT NULL;
ALTER TABLE orders DROP CONSTRAINT orders_payment_method_check;
ALTER TABLE orders ADD CONSTRAINT orders_payment_method_check CHECK(payment_method IN ('cash','online'));
DROP TABLE stringer_payment_settings;
