-- Seed system categories (depth=0)
INSERT INTO categories (slug, name, direction, description, depth, is_system) VALUES
    ('salary',              'Salary',               'credit', 'Regular employment income credited by employer. Look for employer name, "salary".', 0, TRUE),
    ('freelance',           'Freelance',             'credit', 'Freelance or contract payment from a client.', 0, TRUE),
    ('rental_income',       'Rental Income',         'credit', 'Rent received from a tenant.', 0, TRUE),
    ('interest',            'Interest',              'credit', 'Interest earned on savings, FD, or bonds. Look for "Int.Pd", "Interest Credit", "Int Cr".', 0, TRUE),
    ('dividend',            'Dividend',              'credit', 'Dividend from stocks or mutual funds.', 0, TRUE),
    ('cashback',            'Cashback',              'credit', 'Cashback or reward credit. Look for "cashback", "reward" in description.', 0, TRUE),
    ('refund',              'Refund',                'credit', 'Refund for a cancelled order or returned purchase.', 0, TRUE),
    ('tax_refund',          'Tax Refund',            'credit', 'Income tax refund from IT department.', 0, TRUE),
    ('food_drinks',         'Food & Drinks',         'debit',  'Food, restaurants, delivery apps, groceries. Zomato, Swiggy, Blinkit, BigBasket. Named individuals for cooking/food.', 0, TRUE),
    ('transport',           'Transport',             'debit',  'Transportation — fuel, cabs, metro, bus.', 0, TRUE),
    ('utilities',           'Utilities',             'debit',  'Utility bills — electricity, internet, gas/LPG.', 0, TRUE),
    ('rent',                'Rent',                  'debit',  'Monthly housing rent. Look for flat/building name + landlord name or "RENT" in description.', 0, TRUE),
    ('shopping',            'Shopping',              'debit',  'Retail purchases — Amazon, Flipkart, physical stores. Not credit card bill payments.', 0, TRUE),
    ('entertainment',       'Entertainment',         'debit',  'Movies, streaming, events — Netflix, PVR, BookMyShow.', 0, TRUE),
    ('health',              'Health',                'debit',  'Medical expenses — pharmacy, doctor, hospital, diagnostic lab.', 0, TRUE),
    ('education',           'Education',             'debit',  'School/college fees, course payments, EdTech platforms.', 0, TRUE),
    ('insurance',           'Insurance',             'debit',  'Insurance premium — LIC, SBI General, PMSBY, term/health/vehicle insurance.', 0, TRUE),
    ('subscription',        'Subscription',          'debit',  'Recurring subscription services not covered by other categories.', 0, TRUE),
    ('tax_payment',         'Tax Payment',           'debit',  'Tax payments to government — income tax, GST, TDS, advance tax challan.', 0, TRUE),
    ('credit_card_payment', 'Credit Card Payment',   'debit',  'Paying a credit card bill. CRED/CRED.CLUB, Billdesk to Amex/HDFC/Axis card, "CreditCard Payment" prefix. NOT shopping.', 0, TRUE),
    ('bank_charges',        'Bank Charges',          'debit',  'Fees imposed by the bank — MAB penalty (MABchg), SMS alert charges (SMSChg), debit card annual fee (DCARDFEE), card charges + GST, maintenance fee.', 0, TRUE),
    ('investment',          'Investment',            'debit',  'Stock/ETF purchases, direct equity — Zerodha, Groww, Kuvera.', 0, TRUE),
    ('atm_cash',            'ATM / Cash',            'debit',  'ATM cash withdrawal.', 0, TRUE),
    ('househelp',           'House Help',            'debit',  'Domestic worker payments — cook, maid, driver, cleaner. Named individuals paid regularly for household services.', 0, TRUE),
    ('self_transfer',       'Self Transfer',         'both',   'Moving money between own accounts. Sender and receiver are the same person.', 0, TRUE),
    ('household_transfer',  'Household Transfer',    'both',   'Transfer to/from a known household member (spouse, family).', 0, TRUE),
    ('misc',                'Miscellaneous',         'both',   'Genuinely unclassifiable. Use only when nothing else fits. Do not use as default.', 0, TRUE);

-- Seed sub-categories (depth=1)
INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'food_drinks.delivery', 'Delivery', 'debit', 'Food delivery apps — Zomato, Swiggy, Dunzo.', id, 1, TRUE FROM categories WHERE slug = 'food_drinks';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'food_drinks.restaurants', 'Restaurants', 'debit', 'Restaurant dining or cook/food vendor payments.', id, 1, TRUE FROM categories WHERE slug = 'food_drinks';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'food_drinks.groceries', 'Groceries', 'debit', 'Grocery stores — BigBasket, Blinkit, local grocery.', id, 1, TRUE FROM categories WHERE slug = 'food_drinks';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'transport.fuel', 'Fuel', 'debit', 'Petrol/diesel — HP, Indian Oil, BPCL, fuel stations.', id, 1, TRUE FROM categories WHERE slug = 'transport';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'transport.cab', 'Cab / Ride', 'debit', 'Cab or ride-hailing — Uber, Ola, Rapido.', id, 1, TRUE FROM categories WHERE slug = 'transport';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'transport.metro_bus', 'Metro / Bus', 'debit', 'Metro, bus, or public transport passes.', id, 1, TRUE FROM categories WHERE slug = 'transport';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'utilities.electricity', 'Electricity', 'debit', 'Electricity bills — BESCOM, MSEDCL, Tata Power.', id, 1, TRUE FROM categories WHERE slug = 'utilities';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'utilities.internet', 'Internet', 'debit', 'Broadband or mobile data — Jio, Airtel, ACT.', id, 1, TRUE FROM categories WHERE slug = 'utilities';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'utilities.gas_lpg', 'Gas / LPG', 'debit', 'LPG cylinder or piped gas — Indane, HP Gas.', id, 1, TRUE FROM categories WHERE slug = 'utilities';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'investment.sip', 'SIP', 'debit', 'SIP installment — ACH/NACH debit to Indian Clearing Corp (ICCL) or mutual fund house.', id, 1, TRUE FROM categories WHERE slug = 'investment';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'investment.swp', 'SWP', 'credit', 'SWP redemption — systematic withdrawal credit from mutual fund.', id, 1, TRUE FROM categories WHERE slug = 'investment';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'investment.lumpsum', 'Lumpsum', 'debit', 'One-time lumpsum mutual fund purchase.', id, 1, TRUE FROM categories WHERE slug = 'investment';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'investment.redemption', 'Redemption', 'credit', 'One-time mutual fund redemption credit.', id, 1, TRUE FROM categories WHERE slug = 'investment';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'househelp.cook', 'Cook', 'debit', 'Cook payment — named individual paid for cooking services.', id, 1, TRUE FROM categories WHERE slug = 'househelp';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'househelp.maid', 'Maid', 'debit', 'Maid or cleaner payment.', id, 1, TRUE FROM categories WHERE slug = 'househelp';

INSERT INTO categories (slug, name, direction, description, parent_id, depth, is_system)
SELECT 'househelp.driver', 'Driver', 'debit', 'Driver payment.', id, 1, TRUE FROM categories WHERE slug = 'househelp';
