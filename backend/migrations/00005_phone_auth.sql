-- +goose Up
-- Codes remain with the SMS verification provider. Only random challenge hashes
-- and short-lived delivery metadata are stored here; no OTPs or raw IP addresses.
CREATE TABLE auth_challenges (
    token_hash BYTEA PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    phone_number TEXT NOT NULL,
    provider_sid TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '10 minutes',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    processing BOOLEAN NOT NULL DEFAULT false,
    consumed_at TIMESTAMPTZ
);
CREATE INDEX auth_challenges_phone_idx ON auth_challenges(phone_number, created_at DESC);
CREATE INDEX auth_challenges_expiry_idx ON auth_challenges(expires_at);
CREATE TABLE auth_rate_limits (
    key_hash BYTEA NOT NULL CHECK (octet_length(key_hash) = 32),
    window_start BIGINT NOT NULL,
    requests INTEGER NOT NULL CHECK (requests > 0),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (key_hash, window_start)
);
CREATE INDEX auth_rate_limits_expiry_idx ON auth_rate_limits(expires_at);

-- +goose Down
DROP TABLE auth_rate_limits, auth_challenges;
