CREATE TABLE conversation_sessions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id),
    channel     channel_enum NOT NULL DEFAULT 'cli',
    title       TEXT,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata    JSONB NOT NULL DEFAULT '{}'
);

-- System prompt is rebuilt each turn — not stored (would be stale and wasteful).
-- Tool calls are stored as JSON in metadata on role='assistant' messages.
-- Tool results are stored as role='tool' rows with tool_call_id for correlation.
CREATE TABLE conversation_messages (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id   UUID NOT NULL REFERENCES conversation_sessions(id) ON DELETE CASCADE,
    role         msg_role_enum NOT NULL,
    content      TEXT NOT NULL,
    model_used   TEXT,
    tool_name    TEXT,
    tool_call_id TEXT,
    token_count  INT,
    metadata     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user    ON conversation_sessions(user_id);
CREATE INDEX idx_sessions_active  ON conversation_sessions(last_active DESC);
CREATE INDEX idx_messages_session ON conversation_messages(session_id, created_at);
