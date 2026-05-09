CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TYPE account_type_enum AS ENUM (
    'bank_savings', 'bank_current', 'bank_salary',
    'credit_card', 'loan', 'mortgage',
    'demat', 'brokerage', 'ppf', 'epf', 'nps', 'fd',
    'crypto', 'wallet'
);

CREATE TYPE account_class_enum AS ENUM ('asset', 'liability');

CREATE TYPE member_role_enum AS ENUM ('owner', 'joint');

CREATE TYPE txn_direction_enum AS ENUM ('debit', 'credit');

CREATE TYPE payment_mode_enum AS ENUM (
    'upi', 'neft', 'rtgs', 'imps', 'nach',
    'cheque', 'atm', 'pos', 'emi', 'online', 'upi_autopay'
);

CREATE TYPE recurring_mode_enum AS ENUM (
    'nach', 'upi_autopay', 'cc_charge', 'standing_instruction', 'unknown'
);

CREATE TYPE frequency_enum AS ENUM ('daily', 'weekly', 'monthly', 'quarterly', 'annual');

CREATE TYPE transfer_type_enum AS ENUM ('own', 'household', 'investment');

CREATE TYPE tagging_status_enum AS ENUM ('pending', 'auto', 'manual');

CREATE TYPE detection_source_enum AS ENUM ('llm', 'user', 'rule');

CREATE TYPE memory_type_enum AS ENUM ('general', 'tagging_hint', 'recurring_hint', 'preference');

CREATE TYPE channel_enum AS ENUM ('cli', 'slack', 'signal', 'api');

CREATE TYPE msg_role_enum AS ENUM ('system', 'user', 'assistant', 'tool');

CREATE TYPE import_status_enum AS ENUM ('running', 'success', 'partial', 'failed');

CREATE TYPE import_provider_enum AS ENUM ('csv', 'zerodha', 'manual');

CREATE TYPE physical_asset_type_enum AS ENUM (
    'vehicle', 'jewellery', 'real_estate', 'physical_gold', 'other'
);
