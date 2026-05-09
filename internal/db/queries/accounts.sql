-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1 AND deleted_at IS NULL;

-- name: ListAccountsByUser :many
SELECT a.*
FROM accounts a
JOIN account_members am ON am.account_id = a.id
WHERE am.user_id = $1 AND a.deleted_at IS NULL
ORDER BY a.institution, a.name;

-- name: CreateAccount :one
INSERT INTO accounts (institution, external_account_id, name, account_type, currency, is_active, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AddAccountMember :exec
INSERT INTO account_members (account_id, user_id, role) VALUES ($1, $2, $3)
ON CONFLICT (account_id, user_id) DO NOTHING;

-- name: UpsertAccountDetails :exec
INSERT INTO account_details (
    account_id, account_number, ifsc_code, micr_code, swift_code,
    linked_phone, linked_email, branch_name, branch_address, pan_linked, nominee_name
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (account_id) DO UPDATE SET
    account_number = EXCLUDED.account_number,
    ifsc_code      = EXCLUDED.ifsc_code,
    micr_code      = EXCLUDED.micr_code,
    swift_code     = EXCLUDED.swift_code,
    linked_phone   = EXCLUDED.linked_phone,
    linked_email   = EXCLUDED.linked_email,
    branch_name    = EXCLUDED.branch_name,
    branch_address = EXCLUDED.branch_address,
    pan_linked     = EXCLUDED.pan_linked,
    nominee_name   = EXCLUDED.nominee_name,
    updated_at     = NOW();

-- name: GetAccountDetails :one
SELECT * FROM account_details WHERE account_id = $1;
