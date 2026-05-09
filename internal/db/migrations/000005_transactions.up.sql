CREATE TABLE transactions (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id              UUID NOT NULL REFERENCES accounts(id),
    user_id                 UUID NOT NULL REFERENCES users(id),

    idempotency_key         TEXT NOT NULL UNIQUE,
    reference_number        TEXT,

    amount                  BIGINT NOT NULL,
    currency                TEXT NOT NULL DEFAULT 'INR',
    direction               txn_direction_enum NOT NULL,

    original_amount         BIGINT,
    original_currency       TEXT,
    exchange_rate           NUMERIC(20, 8),

    description             TEXT NOT NULL,
    counterparty_name       TEXT,
    counterparty_identifier TEXT,
    payment_mode            payment_mode_enum,

    txn_date                DATE NOT NULL,
    posted_date             DATE,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE transaction_enrichments (
    transaction_id           UUID PRIMARY KEY REFERENCES transactions(id) ON DELETE CASCADE,

    description_normalized   TEXT,
    category_id              UUID REFERENCES categories(id),

    transfer_id              UUID,
    transfer_type            transfer_type_enum,
    refund_of_transaction_id UUID REFERENCES transactions(id),
    recurring_payment_id     UUID,

    notes                    TEXT,
    notes_updated_at         TIMESTAMPTZ,

    embedding                vector(1536),
    embedding_model          TEXT,
    tagging_status           tagging_status_enum NOT NULL DEFAULT 'pending',

    metadata                 JSONB NOT NULL DEFAULT '{}',
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE transaction_labels (
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    label_id       UUID NOT NULL REFERENCES labels(id),
    source         detection_source_enum NOT NULL DEFAULT 'user',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (transaction_id, label_id)
);

CREATE TABLE transaction_splits (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    amount         BIGINT NOT NULL,
    category_id    UUID REFERENCES categories(id),
    notes          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE transaction_split_labels (
    split_id UUID NOT NULL REFERENCES transaction_splits(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES labels(id),
    PRIMARY KEY (split_id, label_id)
);

CREATE INDEX idx_txn_account_id   ON transactions(account_id);
CREATE INDEX idx_txn_user_id      ON transactions(user_id);
CREATE INDEX idx_txn_date         ON transactions(txn_date DESC);
CREATE INDEX idx_txn_payment_mode ON transactions(payment_mode);
CREATE INDEX idx_txn_counterparty ON transactions(counterparty_identifier);

CREATE INDEX idx_enr_category  ON transaction_enrichments(category_id);
CREATE INDEX idx_enr_transfer  ON transaction_enrichments(transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX idx_enr_recurring ON transaction_enrichments(recurring_payment_id) WHERE recurring_payment_id IS NOT NULL;
CREATE INDEX idx_enr_tagging   ON transaction_enrichments(tagging_status);
