CREATE TABLE recurring_payments (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id                 UUID NOT NULL REFERENCES users(id),
    account_id              UUID NOT NULL REFERENCES accounts(id),
    name                    TEXT NOT NULL,
    category_id             UUID REFERENCES categories(id),
    expected_amount         BIGINT,
    amount_variance         BIGINT,
    frequency               frequency_enum NOT NULL,
    approximate_day         SMALLINT,
    payment_mode            recurring_mode_enum,
    counterparty_identifier TEXT,
    counterparty_name       TEXT,
    detection_source        detection_source_enum NOT NULL DEFAULT 'user',
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    last_charged_at         DATE,
    next_expected_at        DATE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE transaction_enrichments
    ADD CONSTRAINT fk_enrichment_recurring
    FOREIGN KEY (recurring_payment_id) REFERENCES recurring_payments(id);

CREATE INDEX idx_recurring_user    ON recurring_payments(user_id);
CREATE INDEX idx_recurring_account ON recurring_payments(account_id);
CREATE INDEX idx_recurring_active  ON recurring_payments(is_active) WHERE is_active = TRUE;
