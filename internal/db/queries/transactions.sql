-- name: InsertTransaction :one
INSERT INTO transactions (
    account_id, user_id, idempotency_key, reference_number,
    amount, currency, direction,
    original_amount, original_currency, exchange_rate,
    description, counterparty_name, counterparty_identifier,
    payment_mode, txn_date, posted_date
) VALUES (
    @account_id, @user_id, @idempotency_key, @reference_number,
    @amount, @currency, @direction,
    @original_amount, @original_currency, @exchange_rate,
    @description, @counterparty_name, @counterparty_identifier,
    @payment_mode, @txn_date, @posted_date
)
RETURNING *;

-- name: InsertTransactionEnrichment :exec
INSERT INTO transaction_enrichments (transaction_id)
VALUES (@transaction_id)
ON CONFLICT DO NOTHING;

-- name: UpdateEnrichment :exec
UPDATE transaction_enrichments SET
    description_normalized   = COALESCE(sqlc.narg(description_normalized), description_normalized),
    category_id              = COALESCE(sqlc.narg(category_id), category_id),
    transfer_id              = COALESCE(sqlc.narg(transfer_id), transfer_id),
    transfer_type            = COALESCE(sqlc.narg(transfer_type), transfer_type),
    refund_of_transaction_id = COALESCE(sqlc.narg(refund_of_transaction_id), refund_of_transaction_id),
    recurring_payment_id     = COALESCE(sqlc.narg(recurring_payment_id), recurring_payment_id),
    notes                    = COALESCE(sqlc.narg(notes), notes),
    notes_updated_at         = CASE WHEN sqlc.narg(notes)::text IS NOT NULL THEN NOW() ELSE notes_updated_at END,
    tagging_status           = COALESCE(sqlc.narg(tagging_status), tagging_status),
    updated_at               = NOW()
WHERE transaction_id = @transaction_id;

-- name: GetTransactionByID :one
SELECT * FROM v_transactions WHERE id = @id;

-- name: ListTransactions :many
SELECT * FROM v_transactions
WHERE user_id = @user_id
  AND (sqlc.narg(from_date)::date IS NULL OR txn_date >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR txn_date <= sqlc.narg(to_date)::date)
  AND (sqlc.narg(account_id)::uuid IS NULL OR account_id = sqlc.narg(account_id)::uuid)
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id)::uuid)
  AND (sqlc.narg(min_amount)::bigint IS NULL OR amount >= sqlc.narg(min_amount)::bigint)
  AND (sqlc.narg(max_amount)::bigint IS NULL OR amount <= sqlc.narg(max_amount)::bigint)
  AND (sqlc.narg(payment_mode)::payment_mode_enum IS NULL OR payment_mode = sqlc.narg(payment_mode)::payment_mode_enum)
  AND (sqlc.narg(counterparty_identifier)::text IS NULL OR counterparty_identifier = sqlc.narg(counterparty_identifier)::text)
  AND (sqlc.narg(direction)::txn_direction_enum IS NULL OR direction = sqlc.narg(direction)::txn_direction_enum)
ORDER BY txn_date DESC, created_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: GetSpendingByCategory :many
SELECT
    c.id AS category_id,
    c.slug AS category_slug,
    c.name AS category_name,
    c.depth,
    SUM(t.amount) AS total_amount,
    COUNT(*) AS txn_count
FROM v_transactions t
JOIN categories c ON c.id = t.category_id
WHERE t.user_id = @user_id
  AND t.direction = 'debit'
  AND t.txn_date >= @from_date
  AND t.txn_date <= @to_date
  AND (sqlc.narg(account_id)::uuid IS NULL OR t.account_id = sqlc.narg(account_id)::uuid)
GROUP BY c.id, c.slug, c.name, c.depth
ORDER BY total_amount DESC;

-- name: ListTransactionsByLabel :many
SELECT vt.* FROM v_transactions vt
JOIN transaction_labels tl ON tl.transaction_id = vt.id
WHERE tl.label_id = @label_id
  AND vt.user_id = @user_id
ORDER BY vt.txn_date DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: GetIdempotencyKeyExists :one
SELECT EXISTS(SELECT 1 FROM transactions WHERE idempotency_key = @idempotency_key);
