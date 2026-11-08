-- +goose Up
CREATE TABLE seller_financial_events (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),stringer_id UUID NOT NULL REFERENCES stringers(id),order_id UUID REFERENCES orders(id),actor_id UUID NOT NULL REFERENCES users(id),
 kind TEXT NOT NULL CHECK(kind IN ('cash_received','cash_refund','adjustment_increase','adjustment_decrease','platform_fee','platform_fee_refund')),
 amount_cents BIGINT NOT NULL CHECK(amount_cents BETWEEN 1 AND 1000000),currency TEXT NOT NULL DEFAULT 'EUR' CHECK(currency='EUR'),
 occurred_at TIMESTAMPTZ NOT NULL,recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),reference TEXT NOT NULL,reason TEXT NOT NULL,
 related_event_id UUID REFERENCES seller_financial_events(id),request_key TEXT NOT NULL,request_hash TEXT NOT NULL,UNIQUE(stringer_id,request_key)
);
CREATE INDEX seller_facts_period ON seller_financial_events(stringer_id,occurred_at);
-- +goose StatementBegin
CREATE FUNCTION reject_financial_rewrite() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Financial facts are append-only; record a linked correction'; END;
$$;
-- +goose StatementEnd
CREATE TRIGGER financial_immutable BEFORE UPDATE OR DELETE ON seller_financial_events FOR EACH ROW EXECUTE FUNCTION reject_financial_rewrite();
-- +goose Down
DROP TABLE seller_financial_events;
DROP FUNCTION reject_financial_rewrite();
