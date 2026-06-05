ALTER TABLE fixed_deposits
    ALTER COLUMN expected_maturity_amount DROP NOT NULL,
    ALTER COLUMN expected_maturity_amount DROP DEFAULT,
    ALTER COLUMN actual_payout_amount     DROP NOT NULL,
    ALTER COLUMN actual_payout_amount     DROP DEFAULT;
