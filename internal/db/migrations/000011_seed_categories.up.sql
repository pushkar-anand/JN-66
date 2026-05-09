-- Seed system categories (depth=0)
INSERT INTO categories (slug, name, direction, depth, is_system) VALUES
    ('salary',        'Salary',        'credit', 0, TRUE),
    ('freelance',     'Freelance',     'credit', 0, TRUE),
    ('rental_income', 'Rental Income', 'credit', 0, TRUE),
    ('interest',      'Interest',      'credit', 0, TRUE),
    ('dividend',      'Dividend',      'credit', 0, TRUE),
    ('cashback',      'Cashback',      'credit', 0, TRUE),
    ('refund',        'Refund',        'credit', 0, TRUE),
    ('tax_refund',    'Tax Refund',    'credit', 0, TRUE),
    ('food_drinks',   'Food & Drinks', 'debit',  0, TRUE),
    ('transport',     'Transport',     'debit',  0, TRUE),
    ('utilities',     'Utilities',     'debit',  0, TRUE),
    ('rent',          'Rent',          'debit',  0, TRUE),
    ('shopping',      'Shopping',      'debit',  0, TRUE),
    ('entertainment', 'Entertainment', 'debit',  0, TRUE),
    ('health',        'Health',        'debit',  0, TRUE),
    ('education',     'Education',     'debit',  0, TRUE),
    ('insurance',     'Insurance',     'debit',  0, TRUE),
    ('subscription',  'Subscription',  'debit',  0, TRUE),
    ('tax_payment',   'Tax Payment',   'debit',  0, TRUE),
    ('transfer',      'Transfer',      NULL,     0, TRUE),
    ('investment',    'Investment',    NULL,     0, TRUE),
    ('atm_cash',      'ATM / Cash',    NULL,     0, TRUE);

-- Seed sub-categories (depth=1)
INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'food_drinks.delivery', 'Delivery', 'debit', id, 1, TRUE FROM categories WHERE slug = 'food_drinks';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'food_drinks.restaurants', 'Restaurants', 'debit', id, 1, TRUE FROM categories WHERE slug = 'food_drinks';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'food_drinks.groceries', 'Groceries', 'debit', id, 1, TRUE FROM categories WHERE slug = 'food_drinks';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'transport.fuel', 'Fuel', 'debit', id, 1, TRUE FROM categories WHERE slug = 'transport';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'transport.cab', 'Cab / Ride', 'debit', id, 1, TRUE FROM categories WHERE slug = 'transport';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'transport.metro_bus', 'Metro / Bus', 'debit', id, 1, TRUE FROM categories WHERE slug = 'transport';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'utilities.electricity', 'Electricity', 'debit', id, 1, TRUE FROM categories WHERE slug = 'utilities';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'utilities.internet', 'Internet', 'debit', id, 1, TRUE FROM categories WHERE slug = 'utilities';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'utilities.gas_lpg', 'Gas / LPG', 'debit', id, 1, TRUE FROM categories WHERE slug = 'utilities';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'investment.sip', 'SIP', NULL, id, 1, TRUE FROM categories WHERE slug = 'investment';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'transfer.own', 'Own Transfer', NULL, id, 1, TRUE FROM categories WHERE slug = 'transfer';

INSERT INTO categories (slug, name, direction, parent_id, depth, is_system)
SELECT 'transfer.household', 'Household', NULL, id, 1, TRUE FROM categories WHERE slug = 'transfer';
