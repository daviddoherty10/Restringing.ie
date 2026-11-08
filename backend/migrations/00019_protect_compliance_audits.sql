-- +goose Up
CREATE TABLE privacy_decision_events (
 id BIGSERIAL PRIMARY KEY,request_id UUID NOT NULL REFERENCES privacy_requests(id),actor_id UUID NOT NULL REFERENCES users(id),
 status TEXT NOT NULL,reason TEXT NOT NULL,details JSONB NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementBegin
CREATE FUNCTION reject_compliance_audit_rewrite() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Compliance audit records are append-only'; END;
$$;
-- +goose StatementEnd
CREATE TRIGGER compliance_audit_immutable BEFORE UPDATE OR DELETE ON compliance_audit FOR EACH ROW EXECUTE FUNCTION reject_compliance_audit_rewrite();
CREATE TRIGGER privacy_decisions_immutable BEFORE UPDATE OR DELETE ON privacy_decision_events FOR EACH ROW EXECUTE FUNCTION reject_compliance_audit_rewrite();
CREATE TRIGGER trader_events_immutable BEFORE UPDATE OR DELETE ON trader_verification_events FOR EACH ROW EXECUTE FUNCTION reject_compliance_audit_rewrite();
CREATE TRIGGER case_events_immutable BEFORE UPDATE OR DELETE ON compliance_case_events FOR EACH ROW EXECUTE FUNCTION reject_compliance_audit_rewrite();
-- +goose Down
DROP TRIGGER case_events_immutable ON compliance_case_events;
DROP TRIGGER trader_events_immutable ON trader_verification_events;
DROP TRIGGER privacy_decisions_immutable ON privacy_decision_events;
DROP TRIGGER compliance_audit_immutable ON compliance_audit;
DROP TABLE privacy_decision_events;
DROP FUNCTION reject_compliance_audit_rewrite();
