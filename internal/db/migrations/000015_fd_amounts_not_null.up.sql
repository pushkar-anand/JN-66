-- Make nullable FD amount columns NOT NULL so pgx can scan them into model.Money (int64).
-- Active FDs have no actual_payout_amount yet; 0 is the correct sentinel.
UPDATE fixed_deposits SET expected_maturity_amount = 0 WHERE expected_maturity_amount IS NULL;
UPDATE fixed_deposits SET actual_payout_amount     = 0 WHERE actual_payout_amount     IS NULL;

ALTER TABLE fixed_deposits
    ALTER COLUMN expected_maturity_amount SET NOT NULL,
    ALTER COLUMN expected_maturity_amount SET DEFAULT 0,
    ALTER COLUMN actual_payout_amount     SET NOT NULL,
    ALTER COLUMN actual_payout_amount     SET DEFAULT 0;
