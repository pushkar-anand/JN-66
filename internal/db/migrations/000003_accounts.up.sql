CREATE TABLE accounts (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    institution         TEXT NOT NULL,
    external_account_id TEXT,
    name                TEXT NOT NULL,
    account_type        account_type_enum NOT NULL,
    account_class       account_class_enum NOT NULL GENERATED ALWAYS AS (
        CASE
            WHEN account_type IN (
                'bank_savings', 'bank_current', 'bank_salary',
                'demat', 'brokerage', 'ppf', 'epf', 'nps',
                'fd', 'crypto', 'wallet'
            )
                THEN 'asset'::account_class_enum
            ELSE 'liability'::account_class_enum
        END
    ) STORED,
    currency            TEXT NOT NULL DEFAULT 'INR',
    current_balance     BIGINT NOT NULL DEFAULT 0,
    balance_as_of       DATE,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ,
    UNIQUE (institution, external_account_id)
);

CREATE TABLE account_members (
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id),
    role       member_role_enum NOT NULL DEFAULT 'owner',
    PRIMARY KEY (account_id, user_id)
);

CREATE TABLE account_details (
    account_id      UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    account_number  TEXT,
    ifsc_code       TEXT,
    micr_code       TEXT,
    swift_code      TEXT,
    linked_phone    TEXT,
    linked_email    TEXT,
    branch_name     TEXT,
    branch_address  TEXT,
    pan_linked      BOOLEAN DEFAULT FALSE,
    nominee_name    TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_type        ON accounts(account_type);
CREATE INDEX idx_accounts_institution ON accounts(institution);
CREATE INDEX idx_account_members_user ON account_members(user_id);
