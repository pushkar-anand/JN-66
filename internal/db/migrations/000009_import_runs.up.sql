CREATE TABLE import_runs (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID NOT NULL REFERENCES users(id),
    account_id     UUID REFERENCES accounts(id),
    provider       import_provider_enum NOT NULL,
    status         import_status_enum NOT NULL DEFAULT 'running',
    source_ref     TEXT,
    rows_parsed    INT NOT NULL DEFAULT 0,
    rows_inserted  INT NOT NULL DEFAULT 0,
    rows_duplicate INT NOT NULL DEFAULT 0,
    rows_failed    INT NOT NULL DEFAULT 0,
    error_detail   TEXT,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at    TIMESTAMPTZ,
    metadata       JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_import_runs_user    ON import_runs(user_id);
CREATE INDEX idx_import_runs_account ON import_runs(account_id);
CREATE INDEX idx_import_runs_status  ON import_runs(status);
