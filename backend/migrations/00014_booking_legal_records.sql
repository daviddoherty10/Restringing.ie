-- +goose Up
CREATE TABLE booking_legal_versions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), stringer_id UUID NOT NULL REFERENCES stringers(id),
 document JSONB NOT NULL, approved_by UUID NOT NULL REFERENCES users(id), approved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 active BOOLEAN NOT NULL DEFAULT true
);
CREATE UNIQUE INDEX booking_legal_current ON booking_legal_versions(stringer_id) WHERE active;
ALTER TABLE quotes ADD COLUMN legal_version_id UUID REFERENCES booking_legal_versions(id);
CREATE TABLE booking_confirmations (
 order_id UUID PRIMARY KEY REFERENCES orders(id), customer_id UUID NOT NULL REFERENCES users(id),
 legal_version_id UUID REFERENCES booking_legal_versions(id), snapshot JSONB NOT NULL,
 acceptance_hash TEXT NOT NULL, early_requested BOOLEAN NOT NULL, work_not_before TIMESTAMPTZ NOT NULL,
 recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE compliance_audit (
 id BIGSERIAL PRIMARY KEY, actor_id UUID REFERENCES users(id), resource_type TEXT NOT NULL,
 resource_id TEXT NOT NULL, action TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementBegin
CREATE FUNCTION protect_booking_legal_record() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='booking_legal_versions' AND TG_OP='UPDATE' AND NEW.id=OLD.id AND NEW.stringer_id=OLD.stringer_id AND NEW.document=OLD.document AND NEW.approved_by=OLD.approved_by AND NEW.approved_at=OLD.approved_at AND OLD.active AND NOT NEW.active THEN RETURN NEW; END IF;
 RAISE EXCEPTION 'Contractual records are immutable';
END $$;
-- +goose StatementEnd
CREATE TRIGGER legal_version_immutable BEFORE UPDATE OR DELETE ON booking_legal_versions FOR EACH ROW EXECUTE FUNCTION protect_booking_legal_record();
CREATE TRIGGER confirmation_immutable BEFORE UPDATE OR DELETE ON booking_confirmations FOR EACH ROW EXECUTE FUNCTION protect_booking_legal_record();
-- +goose Down
DROP TABLE booking_confirmations;
ALTER TABLE quotes DROP COLUMN legal_version_id;
DROP TABLE booking_legal_versions,compliance_audit;
DROP FUNCTION protect_booking_legal_record();
