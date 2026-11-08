-- +goose Up
CREATE TABLE privacy_requests (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id),
 kind TEXT NOT NULL CHECK(kind IN ('access','correction','erasure')), details TEXT NOT NULL CHECK(length(details) BETWEEN 1 AND 2000),
 status TEXT NOT NULL DEFAULT 'submitted' CHECK(status IN ('submitted','in_review','awaiting_identity','extended','fulfilled','refused')),
 outcome TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), due_at TIMESTAMPTZ NOT NULL DEFAULT now()+interval '1 month',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), version INTEGER NOT NULL DEFAULT 1,
 identity_verified_by UUID REFERENCES users(id), export JSONB, retention_basis TEXT NOT NULL DEFAULT '', retain_until DATE,
 request_key TEXT NOT NULL, UNIQUE(user_id,request_key)
);
CREATE INDEX privacy_queue ON privacy_requests(status,due_at);
-- +goose Down
DROP TABLE privacy_requests;
