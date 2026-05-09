CREATE TABLE categories (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    direction  txn_direction_enum,
    parent_id  UUID REFERENCES categories(id),
    depth      SMALLINT NOT NULL DEFAULT 0,
    color      TEXT,
    icon       TEXT,
    is_system  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_parent ON categories(parent_id);
CREATE INDEX idx_categories_depth  ON categories(depth);

CREATE TABLE labels (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID REFERENCES users(id),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    color      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_labels_user ON labels(user_id);
