CREATE TABLE agent_memories (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID REFERENCES users(id),
    content          TEXT NOT NULL,
    memory_type      memory_type_enum NOT NULL DEFAULT 'general',
    detection_source detection_source_enum NOT NULL DEFAULT 'user',
    tags             TEXT[] NOT NULL DEFAULT '{}',
    embedding        vector(1536),
    expires_at       TIMESTAMPTZ,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_memories_user   ON agent_memories(user_id);
CREATE INDEX idx_memories_tags   ON agent_memories USING GIN(tags);
CREATE INDEX idx_memories_active ON agent_memories(is_active) WHERE is_active = TRUE;
