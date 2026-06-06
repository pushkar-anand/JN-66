-- name: CreateFixedDeposit :one
INSERT INTO fixed_deposits (
    user_id, account_id, bank_fd_number,
    principal_amount, interest_rate_bps, tenure_months,
    start_date, maturity_date, expected_maturity_amount,
    interest_payout, auto_renewal_type, renewed_from_id, notes
) VALUES (
    $1, $2, $3,
    $4, $5, $6,
    $7, $8, $9,
    $10, $11, $12, $13
)
RETURNING *;

-- name: GetFixedDeposit :one
SELECT * FROM fixed_deposits WHERE id = @id AND user_id = @user_id;

-- name: ListFixedDeposits :many
SELECT * FROM fixed_deposits
WHERE user_id = @user_id
  AND (sqlc.narg(status)::fd_status_enum IS NULL OR status = sqlc.narg(status)::fd_status_enum)
  AND (sqlc.narg(maturing_before)::date IS NULL OR maturity_date <= sqlc.narg(maturing_before)::date)
ORDER BY maturity_date ASC;

-- name: UpdateFixedDepositStatus :one
UPDATE fixed_deposits
SET status               = @status,
    actual_payout_amount = @actual_payout_amount,
    updated_at           = NOW()
WHERE id = @id AND user_id = @user_id
RETURNING *;
