CREATE TABLE subscription (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    price archive.money,
    billing archive.recurrence,
    renews date,
    website archive.url,
    account uuid REFERENCES bank_account,
    active boolean NOT NULL DEFAULT true,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED
);
CREATE INDEX subscription_search ON subscription USING gin (search);
ALTER TABLE subscription ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON subscription FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON subscription USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE subscription IS 'icon: repeat';
