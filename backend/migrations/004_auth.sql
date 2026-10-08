-- Legacy monitors stay unowned until an operator explicitly assigns them.
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL CHECK (email = lower(btrim(email))),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX sessions_user_id ON sessions(user_id);
CREATE INDEX sessions_expires_at ON sessions(expires_at);

ALTER TABLE monitors ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE RESTRICT;
CREATE INDEX monitors_user_created_at ON monitors(user_id, created_at DESC);

-- Shared across API processes. Never store raw email/IP rate-limit identifiers.
CREATE TABLE auth_rate_limits (
    key_hash BYTEA PRIMARY KEY,
    attempts INTEGER NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX auth_rate_limits_expires_at ON auth_rate_limits(expires_at);
