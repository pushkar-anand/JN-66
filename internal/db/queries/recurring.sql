-- name: ListRecurringPayments :many
SELECT * FROM recurring_payments
WHERE user_id = $1 AND is_active = TRUE
ORDER BY name;

-- name: GetRecurringPaymentByID :one
SELECT * FROM recurring_payments WHERE id = $1;

-- name: CreateRecurringPayment :one
INSERT INTO recurring_payments (
    user_id, account_id, name, category_id,
    expected_amount, amount_variance, frequency, approximate_day,
    payment_mode, counterparty_identifier, counterparty_name,
    detection_source, last_charged_at, next_expected_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, $10, $11,
    $12, $13, $14
) RETURNING *;

-- name: DeactivateRecurringPayment :exec
UPDATE recurring_payments SET is_active = FALSE, updated_at = NOW() WHERE id = $1;

-- name: UpdateRecurringLastCharged :exec
UPDATE recurring_payments SET
    last_charged_at  = $2,
    next_expected_at = $3,
    updated_at       = NOW()
WHERE id = $1;
