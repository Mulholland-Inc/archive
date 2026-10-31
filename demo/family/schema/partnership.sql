CREATE TABLE partnership (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    a uuid REFERENCES person,
    b uuid REFERENCES person,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(notes)) STORED
);
CREATE INDEX partnership_search ON partnership USING gin (search);
ALTER TABLE partnership ENABLE ROW LEVEL SECURITY;
ALTER TABLE partnership FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON partnership FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON partnership USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE partnership IS 'Two people who married or had children together
icon: heart';
COMMENT ON COLUMN partnership.a IS 'One partner, drawn on the left';
COMMENT ON COLUMN partnership.b IS 'The other partner, if known';
COMMENT ON COLUMN partnership.notes IS 'Anything else worth keeping';
