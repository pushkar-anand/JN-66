CREATE TYPE fd_status_enum AS ENUM ('active', 'matured', 'prematurely_closed', 'renewed');
CREATE TYPE fd_payout_enum AS ENUM ('cumulative', 'monthly', 'quarterly', 'annual');
CREATE TYPE fd_renewal_type_enum AS ENUM ('none', 'principal_only', 'principal_and_interest');

CREATE TABLE fixed_deposits (
    id                       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id                  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id               UUID NOT NULL REFERENCES accounts(id),
    bank_fd_number           TEXT,
    principal_amount         BIGINT NOT NULL,
    interest_rate_bps        SMALLINT NOT NULL,
    tenure_months            SMALLINT NOT NULL,
    start_date               DATE NOT NULL,
    maturity_date            DATE NOT NULL,
    expected_maturity_amount BIGINT NOT NULL DEFAULT 0,
    actual_payout_amount     BIGINT NOT NULL DEFAULT 0,
    interest_payout          fd_payout_enum NOT NULL DEFAULT 'cumulative',
    auto_renewal_type        fd_renewal_type_enum NOT NULL DEFAULT 'none',
    status                   fd_status_enum NOT NULL DEFAULT 'active',
    renewed_from_id          UUID REFERENCES fixed_deposits(id),
    notes                    TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fd_user_id      ON fixed_deposits(user_id);
CREATE INDEX idx_fd_account_id   ON fixed_deposits(account_id);
CREATE INDEX idx_fd_maturity     ON fixed_deposits(maturity_date);
CREATE INDEX idx_fd_status       ON fixed_deposits(status);
