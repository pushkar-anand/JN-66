CREATE TABLE zerodha_equity_holdings (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id       UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    tradingsymbol    TEXT NOT NULL,
    exchange         TEXT NOT NULL,
    isin             TEXT,
    quantity         INT NOT NULL,
    avg_price_paise  BIGINT NOT NULL,
    last_price_paise BIGINT NOT NULL,
    pnl_paise        BIGINT NOT NULL,
    day_change_paise BIGINT NOT NULL DEFAULT 0,
    synced_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, tradingsymbol, exchange)
);

CREATE INDEX idx_zerodha_eq_user    ON zerodha_equity_holdings(user_id);
CREATE INDEX idx_zerodha_eq_account ON zerodha_equity_holdings(account_id);

CREATE TABLE zerodha_mf_holdings (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id    UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    folio         TEXT NOT NULL,
    fund          TEXT NOT NULL,
    tradingsymbol TEXT NOT NULL,
    units         NUMERIC(18, 4) NOT NULL,
    avg_nav_paise BIGINT NOT NULL,
    nav_paise     BIGINT NOT NULL,
    pnl_paise     BIGINT NOT NULL,
    synced_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, folio, tradingsymbol)
);

CREATE INDEX idx_zerodha_mf_user    ON zerodha_mf_holdings(user_id);
CREATE INDEX idx_zerodha_mf_account ON zerodha_mf_holdings(account_id);
