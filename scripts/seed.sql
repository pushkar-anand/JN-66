-- Seed data for local development and verification.
-- Run after all migrations: psql $DATABASE_URL -f scripts/seed.sql
-- Safe to re-run: uses INSERT ... ON CONFLICT DO NOTHING where possible.

BEGIN;

-- ── Users ─────────────────────────────────────────────────────────────────────

INSERT INTO users (id, name, email, phone, timezone) VALUES
    ('a1000000-0000-0000-0000-000000000001', 'Alice', 'alice@example.com', '+919876543210', 'Asia/Kolkata'),
    ('a2000000-0000-0000-0000-000000000002', 'Bob',   'bob@example.com',   '+919876543211', 'Asia/Kolkata')
ON CONFLICT (email) DO NOTHING;

-- ── Accounts ──────────────────────────────────────────────────────────────────

INSERT INTO accounts (id, institution, external_account_id, name, account_type, currency, is_active) VALUES
    ('b1000000-0000-0000-0000-000000000001', 'hdfc',   'HDFC****1234', 'HDFC Savings ****1234',       'bank_savings', 'INR', TRUE),
    ('b2000000-0000-0000-0000-000000000002', 'hdfc',   'HDFCCC****5678', 'HDFC Credit Card ****5678', 'credit_card',  'INR', TRUE),
    ('b3000000-0000-0000-0000-000000000003', 'sbi',    'SBI****9012',  'SBI Savings ****9012',        'bank_savings', 'INR', TRUE)
ON CONFLICT (institution, external_account_id) DO NOTHING;

-- Account members
INSERT INTO account_members (account_id, user_id, role) VALUES
    ('b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001', 'owner'),
    ('b2000000-0000-0000-0000-000000000002', 'a1000000-0000-0000-0000-000000000001', 'owner'),
    ('b3000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000002', 'owner')
ON CONFLICT DO NOTHING;

-- ── Transactions: Alice's HDFC Savings (April–May 2026) ────────────────────
-- Amounts in paise (INR × 100)

INSERT INTO transactions (id, account_id, user_id, idempotency_key, amount, currency, direction,
    description, counterparty_name, counterparty_identifier, payment_mode, txn_date) VALUES

-- Salary credit
('c0100000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260401-salary',
 15000000, 'INR', 'credit',
 'SALARY CREDIT APR 2026', 'Acme Tech Pvt Ltd', NULL, 'neft', '2026-04-01'),

-- UPI — Zomato food delivery
('c0200000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260402-zomato1',
 49900, 'INR', 'debit',
 'UPI/Zomato/Order#8821', 'Zomato Payments', 'zomato@axisbank', 'upi', '2026-04-02'),

-- UPI — Swiggy
('c0300000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260405-swiggy',
 34500, 'INR', 'debit',
 'UPI/Swiggy/Order#4412', 'Swiggy', 'swiggy@icici', 'upi', '2026-04-05'),

-- NACH — SIP deduction
('c0400000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260405-sip',
 500000, 'INR', 'debit',
 'NACH DR-PARAG PARIKH FLEXI CAP', 'PPFAS MF', 'ppfas@hdfcbank', 'nach', '2026-04-05'),

-- UPI — Dunzo / groceries
('c0500000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260406-blinkit',
 128000, 'INR', 'debit',
 'UPI/Blinkit/Order#9934', 'Blinkit', 'blinkit@paytm', 'upi', '2026-04-06'),

-- ATM cash
('c0600000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260408-atm',
 500000, 'INR', 'debit',
 'ATM WDL HDFC ATM MG ROAD', NULL, NULL, 'atm', '2026-04-08'),

-- IMPS — Rent transfer to partner (household transfer)
-- transfer_id links this with Bob's credit below
('c0700000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260410-rent-transfer',
 1500000, 'INR', 'debit',
 'IMPS/RENT SHARE APR/Bob', 'Bob', 'bob@sbi', 'imps', '2026-04-10'),

-- UPI — Ola cab
('c0800000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260412-ola',
 32000, 'INR', 'debit',
 'UPI/OLA/Trip#7712', 'Ola Cabs', 'ola@olamoney', 'upi', '2026-04-12'),

-- NEFT — Electricity bill
('c0900000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260415-electricity',
 325000, 'INR', 'debit',
 'NEFT/BESCOM/BP#334455', 'BESCOM', NULL, 'neft', '2026-04-15'),

-- UPI — Amazon (shopping)
('c1000000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260418-amazon',
 149900, 'INR', 'debit',
 'UPI/Amazon/Order#B07XZ', 'Amazon Seller Services', 'amazon@razorpay', 'upi', '2026-04-18'),

-- HDFC CC bill payment (own transfer from savings → CC)
('c1100000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260420-cc-payment',
 1200000, 'INR', 'debit',
 'BILLPAY/HDFC CC/****5678', NULL, NULL, 'neft', '2026-04-20'),

-- Zomato refund (credit)
('c1200000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260421-zomato-refund',
 49900, 'INR', 'credit',
 'REFUND/Zomato/Order#8821', 'Zomato Payments', 'zomato@axisbank', 'upi', '2026-04-21'),

-- Forex — Netflix USD billed in INR
('c1300000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260422-netflix-forex',
 134900, 'INR', 'debit',
 'NETFLIX.COM INR CONV USD', 'Netflix', 'netflix@razorpay', 'online', '2026-04-22'),

-- UPI — Petrol
('c1400000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260425-petrol',
 320000, 'INR', 'debit',
 'UPI/HPCL/Station#221', 'HP Petrol Pump', 'hpcl@hdfcbank', 'upi', '2026-04-25'),

-- May salary
('c1500000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260501-salary',
 15000000, 'INR', 'credit',
 'SALARY CREDIT MAY 2026', 'Acme Tech Pvt Ltd', NULL, 'neft', '2026-05-01'),

-- UPI — Zomato again
('c1600000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260503-zomato2',
 56800, 'INR', 'debit',
 'UPI/Zomato/Order#9103', 'Zomato Payments', 'zomato@axisbank', 'upi', '2026-05-03'),

-- May SIP
('c1700000-0000-0000-0000-000000000001',
 'b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001',
 'seed-hdfc-savings-20260505-sip',
 500000, 'INR', 'debit',
 'NACH DR-PARAG PARIKH FLEXI CAP', 'PPFAS MF', 'ppfas@hdfcbank', 'nach', '2026-05-05')

ON CONFLICT (idempotency_key) DO NOTHING;

-- ── Transactions: Bob's SBI Savings ─────────────────────────────────────────

INSERT INTO transactions (id, account_id, user_id, idempotency_key, amount, currency, direction,
    description, counterparty_name, counterparty_identifier, payment_mode, txn_date) VALUES

-- Salary credit to Bob
('d0100000-0000-0000-0000-000000000001',
 'b3000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000002',
 'seed-sbi-savings-20260401-salary',
 12000000, 'INR', 'credit',
 'SALARY CREDIT APR 2026', 'Beta Corp Pvt Ltd', NULL, 'neft', '2026-04-01'),

-- Rent share received from Alice
('d0200000-0000-0000-0000-000000000001',
 'b3000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000002',
 'seed-sbi-savings-20260410-rent-received',
 1500000, 'INR', 'credit',
 'IMPS/RENT SHARE APR/Alice', 'Alice', 'alice@hdfc', 'imps', '2026-04-10'),

-- Rent paid to landlord
('d0300000-0000-0000-0000-000000000001',
 'b3000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000002',
 'seed-sbi-savings-20260410-rent-paid',
 3000000, 'INR', 'debit',
 'NEFT/RENT APR/Ramesh Kumar', 'Ramesh Kumar', NULL, 'neft', '2026-04-10'),

-- UPI — Swiggy
('d0400000-0000-0000-0000-000000000001',
 'b3000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000002',
 'seed-sbi-savings-20260407-swiggy',
 42100, 'INR', 'debit',
 'UPI/Swiggy/Order#5521', 'Swiggy', 'swiggy@icici', 'upi', '2026-04-07'),

-- Metro / bus
('d0500000-0000-0000-0000-000000000001',
 'b3000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000002',
 'seed-sbi-savings-20260415-metro',
 15000, 'INR', 'debit',
 'UPI/BMTCMETRO/Topup', 'BMTC Metro', 'bmtc@sbi', 'upi', '2026-04-15')

ON CONFLICT (idempotency_key) DO NOTHING;

-- ── Transaction enrichments (blank rows for all transactions) ─────────────────

INSERT INTO transaction_enrichments (transaction_id)
SELECT id FROM transactions
WHERE id IN (
    'c0100000-0000-0000-0000-000000000001',
    'c0200000-0000-0000-0000-000000000001',
    'c0300000-0000-0000-0000-000000000001',
    'c0400000-0000-0000-0000-000000000001',
    'c0500000-0000-0000-0000-000000000001',
    'c0600000-0000-0000-0000-000000000001',
    'c0700000-0000-0000-0000-000000000001',
    'c0800000-0000-0000-0000-000000000001',
    'c0900000-0000-0000-0000-000000000001',
    'c1000000-0000-0000-0000-000000000001',
    'c1100000-0000-0000-0000-000000000001',
    'c1200000-0000-0000-0000-000000000001',
    'c1300000-0000-0000-0000-000000000001',
    'c1400000-0000-0000-0000-000000000001',
    'c1500000-0000-0000-0000-000000000001',
    'c1600000-0000-0000-0000-000000000001',
    'c1700000-0000-0000-0000-000000000001',
    'd0100000-0000-0000-0000-000000000001',
    'd0200000-0000-0000-0000-000000000001',
    'd0300000-0000-0000-0000-000000000001',
    'd0400000-0000-0000-0000-000000000001',
    'd0500000-0000-0000-0000-000000000001'
)
ON CONFLICT DO NOTHING;

-- ── Enrich key transactions ───────────────────────────────────────────────────

-- Salary → category: salary
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'salary'),
    tagging_status = 'manual'
WHERE transaction_id IN (
    'c0100000-0000-0000-0000-000000000001',
    'c1500000-0000-0000-0000-000000000001',
    'd0100000-0000-0000-0000-000000000001'
);

-- Zomato debits → food_drinks.delivery
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'food_drinks.delivery'),
    tagging_status = 'manual'
WHERE transaction_id IN (
    'c0200000-0000-0000-0000-000000000001',
    'c1600000-0000-0000-0000-000000000001',
    'd0400000-0000-0000-0000-000000000001'
);

-- Swiggy → food_drinks.delivery
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'food_drinks.delivery'),
    tagging_status = 'manual'
WHERE transaction_id = 'c0300000-0000-0000-0000-000000000001';

-- SIP → investment.sip
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'investment.sip'),
    tagging_status = 'manual'
WHERE transaction_id IN (
    'c0400000-0000-0000-0000-000000000001',
    'c1700000-0000-0000-0000-000000000001'
);

-- Groceries → food_drinks.groceries
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'food_drinks.groceries'),
    tagging_status = 'manual'
WHERE transaction_id = 'c0500000-0000-0000-0000-000000000001';

-- ATM → atm_cash
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'atm_cash'),
    tagging_status = 'manual'
WHERE transaction_id = 'c0600000-0000-0000-0000-000000000001';

-- Household transfer (both sides share a transfer_id, type = household)
UPDATE transaction_enrichments SET
    transfer_id = 'e1000000-0000-0000-0000-000000000001',
    transfer_type = 'household',
    category_id = (SELECT id FROM categories WHERE slug = 'transfer.household'),
    tagging_status = 'manual'
WHERE transaction_id IN (
    'c0700000-0000-0000-0000-000000000001',
    'd0200000-0000-0000-0000-000000000001'
);

-- Ola → transport.cab
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'transport.cab'),
    tagging_status = 'manual'
WHERE transaction_id = 'c0800000-0000-0000-0000-000000000001';

-- Electricity → utilities.electricity
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'utilities.electricity'),
    tagging_status = 'manual'
WHERE transaction_id = 'c0900000-0000-0000-0000-000000000001';

-- Amazon → shopping
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'shopping'),
    tagging_status = 'manual'
WHERE transaction_id = 'c1000000-0000-0000-0000-000000000001';

-- CC bill payment → own transfer
UPDATE transaction_enrichments SET
    transfer_id = 'e2000000-0000-0000-0000-000000000001',
    transfer_type = 'own',
    category_id = (SELECT id FROM categories WHERE slug = 'transfer.own'),
    tagging_status = 'manual'
WHERE transaction_id = 'c1100000-0000-0000-0000-000000000001';

-- Zomato refund links to original Zomato debit
UPDATE transaction_enrichments SET
    refund_of_transaction_id = 'c0200000-0000-0000-0000-000000000001',
    category_id = (SELECT id FROM categories WHERE slug = 'refund'),
    tagging_status = 'manual'
WHERE transaction_id = 'c1200000-0000-0000-0000-000000000001';

-- Netflix → subscription
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'subscription'),
    tagging_status = 'manual',
    notes = 'Netflix monthly subscription billed in USD, converted to INR'
WHERE transaction_id = 'c1300000-0000-0000-0000-000000000001';

-- Petrol → transport.fuel
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'transport.fuel'),
    tagging_status = 'manual'
WHERE transaction_id = 'c1400000-0000-0000-0000-000000000001';

-- Rent paid → rent
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'rent'),
    tagging_status = 'manual'
WHERE transaction_id = 'd0300000-0000-0000-0000-000000000001';

-- Metro → transport.metro_bus
UPDATE transaction_enrichments SET
    category_id = (SELECT id FROM categories WHERE slug = 'transport.metro_bus'),
    tagging_status = 'manual'
WHERE transaction_id = 'd0500000-0000-0000-0000-000000000001';

-- ── Recurring payment rules ───────────────────────────────────────────────────

INSERT INTO recurring_payments (
    id, user_id, account_id, name, category_id,
    expected_amount, amount_variance, frequency, approximate_day,
    payment_mode, counterparty_identifier, counterparty_name,
    detection_source, last_charged_at, next_expected_at
) VALUES
(
    'f1000000-0000-0000-0000-000000000001',
    'a1000000-0000-0000-0000-000000000001',
    'b1000000-0000-0000-0000-000000000001',
    'PPFAS SIP',
    (SELECT id FROM categories WHERE slug = 'investment.sip'),
    500000, 0, 'monthly', 5,
    'nach', 'ppfas@hdfcbank', 'PPFAS MF',
    'user', '2026-05-05', '2026-06-05'
),
(
    'f2000000-0000-0000-0000-000000000001',
    'a1000000-0000-0000-0000-000000000001',
    'b1000000-0000-0000-0000-000000000001',
    'Netflix Subscription',
    (SELECT id FROM categories WHERE slug = 'subscription'),
    64900, 5000, 'monthly', 22,
    'cc_charge', 'netflix@razorpay', 'Netflix',
    'user', '2026-04-22', '2026-05-22'
)
ON CONFLICT DO NOTHING;

-- Link the SIP transactions to the recurring payment rule
UPDATE transaction_enrichments SET
    recurring_payment_id = 'f1000000-0000-0000-0000-000000000001'
WHERE transaction_id IN (
    'c0400000-0000-0000-0000-000000000001',
    'c1700000-0000-0000-0000-000000000001'
);

UPDATE transaction_enrichments SET
    recurring_payment_id = 'f2000000-0000-0000-0000-000000000001'
WHERE transaction_id = 'c1300000-0000-0000-0000-000000000001';

-- ── Agent memories ────────────────────────────────────────────────────────────

INSERT INTO agent_memories (user_id, content, memory_type, detection_source, tags) VALUES
(
    'a1000000-0000-0000-0000-000000000001',
    'zomato@axisbank is Zomato food delivery — always tag as food_drinks.delivery',
    'tagging_hint', 'user', ARRAY['zomato', 'food', 'delivery', 'upi']
),
(
    'a1000000-0000-0000-0000-000000000001',
    'PPFAS SIP of ₹5000 via NACH on 5th of every month from HDFC Savings',
    'recurring_hint', 'user', ARRAY['sip', 'investment', 'ppfas', 'nach']
),
(
    NULL,
    'Household rent is ₹30,000/month. Alice transfers ₹15,000 share to Bob who pays the full rent to landlord Ramesh Kumar.',
    'general', 'user', ARRAY['rent', 'household', 'transfer', 'priya']
);

COMMIT;
