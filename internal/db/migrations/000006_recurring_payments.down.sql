ALTER TABLE transaction_enrichments DROP CONSTRAINT IF EXISTS fk_enrichment_recurring;
DROP INDEX IF EXISTS idx_recurring_active;
DROP INDEX IF EXISTS idx_recurring_account;
DROP INDEX IF EXISTS idx_recurring_user;
DROP TABLE IF EXISTS recurring_payments;
