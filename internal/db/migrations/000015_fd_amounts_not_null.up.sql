-- Make nullable FD amount columns NOT NULL so pgx can scan them into model.Money (int64).
-- Active FDs have no actual_payout_amount yet; 0 is the correct sentinel.
--
-- NOTE: 000014 was patched after this migration was written to declare these columns as
-- NOT NULL DEFAULT 0 from the start. This migration is therefore a no-op on clean installs
-- (the UPDATEs find no NULLs; SET NOT NULL on an already-NOT-NULL column is harmless in PG).
-- It is kept for databases that ran 000014 before the patch was applied.
UPDATE fixed_deposits SET expected_maturity_amount = 0 WHERE expected_maturity_amount IS NULL;
UPDATE fixed_deposits SET actual_payout_amount     = 0 WHERE actual_payout_amount     IS NULL;

ALTER TABLE fixed_deposits
    ALTER COLUMN expected_maturity_amount SET NOT NULL,
    ALTER COLUMN expected_maturity_amount SET DEFAULT 0,
    ALTER COLUMN actual_payout_amount     SET NOT NULL,
    ALTER COLUMN actual_payout_amount     SET DEFAULT 0;
