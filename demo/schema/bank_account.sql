CREATE TYPE account_kind AS ENUM ('checking', 'savings', 'credit', 'investment', 'retirement');
CREATE TABLE bank_account (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    institution text,
    kind account_kind,
    routing archive.us_routing,
    last4 text,
    balance archive.money,
    opened date,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, institution)) STORED
);
CREATE INDEX bank_account_search ON bank_account USING gin (search);
ALTER TABLE bank_account ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_account FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON bank_account USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE bank_account IS 'A bank, savings, credit or investment account
icon: landmark';
COMMENT ON COLUMN bank_account.name IS 'Your name for the account';
COMMENT ON COLUMN bank_account.institution IS 'Bank or broker';
COMMENT ON COLUMN bank_account.kind IS 'Type of account';
COMMENT ON COLUMN bank_account.routing IS 'ABA routing number of the bank';
COMMENT ON COLUMN bank_account.last4 IS 'Last four digits of the account number';
COMMENT ON COLUMN bank_account.balance IS 'Last known balance';
COMMENT ON COLUMN bank_account.opened IS 'Date opened';
