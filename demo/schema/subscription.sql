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
COMMENT ON TABLE subscription IS 'Something you pay for on a schedule';
COMMENT ON COLUMN subscription.name IS 'Service name';
COMMENT ON COLUMN subscription.price IS 'Price per billing period';
COMMENT ON COLUMN subscription.billing IS 'Billing schedule as an RRULE';
COMMENT ON COLUMN subscription.renews IS 'Next renewal date';
COMMENT ON COLUMN subscription.website IS 'Where to manage or cancel it';
COMMENT ON COLUMN subscription.account IS 'Account it is paid from';
COMMENT ON COLUMN subscription.active IS 'Still subscribed';
