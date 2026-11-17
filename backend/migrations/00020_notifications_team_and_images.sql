-- +goose Up
ALTER TABLE users ADD COLUMN job_sms_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN job_sms_updated_at TIMESTAMPTZ;
CREATE TABLE notification_deliveries (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 notification_id UUID NOT NULL UNIQUE REFERENCES notifications(id) ON DELETE CASCADE,
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','sending','accepted','sent','delivered','failed','unknown','cancelled')),
 attempts INT NOT NULL DEFAULT 0,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 started_at TIMESTAMPTZ,
 provider_sid TEXT UNIQUE,
 error_code TEXT NOT NULL DEFAULT '',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notification_deliveries_pending ON notification_deliveries(next_attempt_at) WHERE state='pending';
CREATE TABLE staff_invitations (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 stringer_id UUID NOT NULL REFERENCES stringers(id),
 phone_number TEXT NOT NULL,
 invited_by UUID NOT NULL REFERENCES users(id),
 expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+interval '7 days',
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','accepted','revoked')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(stringer_id,phone_number)
);
CREATE TABLE team_events (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 stringer_id UUID NOT NULL REFERENCES stringers(id),
 actor_id UUID NOT NULL REFERENCES users(id),
 action TEXT NOT NULL CHECK(action IN ('invite','accept','revoke','remove')),
 subject TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX team_events_recent ON team_events(stringer_id,created_at);
CREATE INDEX notification_deliveries_started ON notification_deliveries(started_at) WHERE started_at IS NOT NULL;
CREATE INDEX staff_invitations_phone ON staff_invitations(phone_number) WHERE state='pending';
CREATE TRIGGER team_events_immutable BEFORE UPDATE OR DELETE ON team_events FOR EACH ROW EXECUTE FUNCTION reject_compliance_audit_rewrite();
CREATE TABLE business_images (
 stringer_id UUID NOT NULL REFERENCES stringers(id),
 slot TEXT NOT NULL CHECK(slot IN ('profile','banner')),
 data BYTEA NOT NULL CHECK(octet_length(data)<=1048576),
 version UUID NOT NULL DEFAULT gen_random_uuid(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(stringer_id,slot)
);
-- +goose Down
DROP TABLE business_images,team_events,staff_invitations,notification_deliveries;
ALTER TABLE users DROP COLUMN job_sms_updated_at, DROP COLUMN job_sms_enabled;
