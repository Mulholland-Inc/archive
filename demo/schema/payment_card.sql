CREATE TABLE payment_card (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    number archive.card,
    expires text,
    account uuid REFERENCES bank_account,
    holder uuid REFERENCES person,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED
);
CREATE INDEX payment_card_search ON payment_card USING gin (search);
ALTER TABLE payment_card ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_card FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON payment_card FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON payment_card USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE payment_card IS 'icon: credit-card';
