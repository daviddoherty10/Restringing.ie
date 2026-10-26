-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    phone_number TEXT NOT NULL UNIQUE,
    phone_verified_at TIMESTAMPTZ NOT NULL,

    first_name TEXT,
    last_name TEXT,
    email TEXT,

    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'closed')),

    platform_role TEXT NOT NULL DEFAULT 'user'
        CHECK (platform_role IN ('user', 'admin')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE users;
