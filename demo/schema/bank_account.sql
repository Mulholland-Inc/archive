CREATE TYPE account_kind AS ENUM ('checking', 'savings', 'credit', 'investment', 'pension');
CREATE TABLE bank_account (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    institution text,
    kind account_kind,
    iban archive.iban,
    bic archive.bic,
    balance archive.money,
    opened date,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, institution)) STORED
);
CREATE INDEX bank_account_search ON bank_account USING gin (search);
ALTER TABLE bank_account ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_account FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON bank_account FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON bank_account USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE bank_account IS 'icon: landmark';
