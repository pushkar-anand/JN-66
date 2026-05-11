CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username       TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    email          TEXT UNIQUE,
    phone          TEXT,
    date_of_birth  DATE,
    timezone       TEXT NOT NULL DEFAULT 'Asia/Kolkata',
    preferences    JSONB NOT NULL DEFAULT '{}',
    api_key_hash   BYTEA NOT NULL UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
