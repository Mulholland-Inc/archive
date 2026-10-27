CREATE TYPE payment_category AS ENUM ('income', 'rent', 'groceries', 'eating_out', 'transport', 'utilities', 'insurance', 'subscriptions', 'shopping', 'health', 'travel', 'gifts', 'savings', 'taxes', 'other');
CREATE TABLE payment (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    booked date NOT NULL,
    amount archive.money NOT NULL,
    counterparty text,
    description text,
    category payment_category,
    account uuid REFERENCES bank_account,
    receipt uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(counterparty, description)) STORED
);
CREATE INDEX payment_search ON payment USING gin (search);
ALTER TABLE payment ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON payment FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON payment USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE payment IS 'Money in or out of an account; negative amounts are spending
icon: arrow-left-right';
COMMENT ON COLUMN payment.booked IS 'Booking date';
COMMENT ON COLUMN payment.amount IS 'Amount, negative when you paid';
COMMENT ON COLUMN payment.counterparty IS 'Who paid you or whom you paid';
COMMENT ON COLUMN payment.description IS 'Description from the bank';
COMMENT ON COLUMN payment.category IS 'Spending category';
COMMENT ON COLUMN payment.account IS 'Account it was booked on';
COMMENT ON COLUMN payment.receipt IS 'Receipt or invoice file';
