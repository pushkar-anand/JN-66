-- name: UpsertZerodhaToken :exec
INSERT INTO zerodha_tokens (user_id, access_token, expires_at)
VALUES (@user_id, @access_token, @expires_at)
ON CONFLICT (user_id) DO UPDATE SET
    access_token = EXCLUDED.access_token,
    expires_at   = EXCLUDED.expires_at,
    updated_at   = NOW();

-- name: GetZerodhaToken :one
SELECT * FROM zerodha_tokens WHERE user_id = @user_id;

-- name: DeleteZerodhaToken :exec
DELETE FROM zerodha_tokens WHERE user_id = @user_id;

-- name: GetZerodhaEquitySyncedAt :one
SELECT MAX(synced_at) AS synced_at FROM zerodha_equity_holdings WHERE account_id = @account_id;

-- name: DeleteZerodhaEquityHoldings :exec
DELETE FROM zerodha_equity_holdings WHERE account_id = @account_id;

-- name: InsertZerodhaEquityHolding :exec
INSERT INTO zerodha_equity_holdings (
    user_id, account_id, tradingsymbol, exchange, isin,
    quantity, avg_price_paise, last_price_paise, pnl_paise, day_change_paise
) VALUES (
    @user_id, @account_id, @tradingsymbol, @exchange, @isin,
    @quantity, @avg_price_paise, @last_price_paise, @pnl_paise, @day_change_paise
);

-- name: ListZerodhaEquityHoldings :many
SELECT * FROM zerodha_equity_holdings
WHERE user_id = @user_id
ORDER BY tradingsymbol;

-- name: GetZerodhaEquitySummary :one
SELECT
    COUNT(*)                         AS holding_count,
    SUM(quantity * last_price_paise) AS current_value_paise,
    SUM(quantity * avg_price_paise)  AS invested_value_paise,
    SUM(pnl_paise)                   AS total_pnl_paise,
    MAX(synced_at)                   AS synced_at
FROM zerodha_equity_holdings
WHERE user_id = @user_id;

-- name: GetZerodhaEquityHoldingsByType :many
SELECT
    CASE WHEN LEFT(tradingsymbol, 3) = 'SGB' THEN 'sgb' ELSE 'equity' END AS holding_type,
    COUNT(*)                         AS holding_count,
    SUM(quantity * last_price_paise) AS current_value_paise,
    SUM(quantity * avg_price_paise)  AS invested_value_paise,
    SUM(pnl_paise)                   AS total_pnl_paise
FROM zerodha_equity_holdings
WHERE user_id = @user_id
GROUP BY holding_type;

-- name: GetZerodhaMFSyncedAt :one
SELECT MAX(synced_at) AS synced_at FROM zerodha_mf_holdings WHERE account_id = @account_id;

-- name: DeleteZerodhaMFHoldings :exec
DELETE FROM zerodha_mf_holdings WHERE account_id = @account_id;

-- name: InsertZerodhaMFHolding :exec
INSERT INTO zerodha_mf_holdings (
    user_id, account_id, folio, fund, tradingsymbol,
    units, avg_nav_paise, nav_paise, pnl_paise
) VALUES (
    @user_id, @account_id, @folio, @fund, @tradingsymbol,
    @units, @avg_nav_paise, @nav_paise, @pnl_paise
);

-- name: ListZerodhaMFHoldings :many
SELECT * FROM zerodha_mf_holdings
WHERE user_id = @user_id
ORDER BY fund;

-- name: GetZerodhaMFSummary :one
SELECT
    COUNT(*)                   AS holding_count,
    SUM(units * nav_paise)     AS current_value_paise,
    SUM(units * avg_nav_paise) AS invested_value_paise,
    SUM(pnl_paise)             AS total_pnl_paise,
    MAX(synced_at)             AS synced_at
FROM zerodha_mf_holdings
WHERE user_id = @user_id;
