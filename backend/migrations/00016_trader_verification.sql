-- +goose Up
CREATE TABLE trader_verifications (
 stringer_id UUID PRIMARY KEY REFERENCES stringers(id), document JSONB NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('submitted','needs_info','verified','rejected','suspended')),
 version INTEGER NOT NULL DEFAULT 1, reason TEXT NOT NULL DEFAULT '',
 reviewed_by UUID REFERENCES users(id), reviewed_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE trader_verification_events (
 id BIGSERIAL PRIMARY KEY,stringer_id UUID NOT NULL REFERENCES stringers(id),actor_id UUID NOT NULL REFERENCES users(id),
 status TEXT NOT NULL, document JSONB NOT NULL, reason TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE trader_verification_events,trader_verifications;
