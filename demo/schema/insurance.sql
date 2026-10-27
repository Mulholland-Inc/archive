CREATE TYPE insurance_kind AS ENUM ('health', 'home', 'contents', 'liability', 'travel', 'car', 'life', 'legal');
CREATE TABLE insurance (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    provider text,
    kind insurance_kind,
    policy_number text,
    premium archive.money,
    deductible archive.money,
    renews date,
    policy uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, provider, policy_number)) STORED
);
CREATE INDEX insurance_search ON insurance USING gin (search);
ALTER TABLE insurance ENABLE ROW LEVEL SECURITY;
ALTER TABLE insurance FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON insurance FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON insurance USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE insurance IS 'An insurance policy';
COMMENT ON COLUMN insurance.name IS 'Your name for the policy';
COMMENT ON COLUMN insurance.provider IS 'Insurer';
COMMENT ON COLUMN insurance.kind IS 'What it covers';
COMMENT ON COLUMN insurance.policy_number IS 'Policy number';
COMMENT ON COLUMN insurance.premium IS 'Monthly premium';
COMMENT ON COLUMN insurance.deductible IS 'Own risk per year or claim';
COMMENT ON COLUMN insurance.renews IS 'Renewal or cancellation date';
COMMENT ON COLUMN insurance.policy IS 'Policy document';
