-- +goose Up
-- Opaque sessions issued after the existing phone OTP flow verifies ownership.
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);
CREATE INDEX sessions_user_idx ON sessions(user_id);

CREATE TABLE stringers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
    area TEXT NOT NULL DEFAULT '' CHECK (length(area) <= 120),
    county TEXT NOT NULL DEFAULT '' CHECK (length(county) <= 120),
    approval_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (approval_status IN ('pending','approved','rejected','suspended')),
    accepting_orders BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stringers_public_idx ON stringers(approval_status,county,id);
CREATE TABLE stringer_members (
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role TEXT NOT NULL CHECK (role IN ('owner','staff')),
    PRIMARY KEY (stringer_id,user_id)
);
CREATE INDEX stringer_members_user_idx ON stringer_members(user_id,stringer_id);
CREATE TABLE stringer_approval_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stringer_id UUID NOT NULL REFERENCES stringers(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','suspended')),
    reason TEXT NOT NULL DEFAULT '' CHECK (length(reason) <= 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE stringer_approval_events, stringer_members, stringers, sessions;
