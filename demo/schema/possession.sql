CREATE TABLE possession (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    brand text,
    category text,
    purchased date,
    price archive.money,
    warranty_until date,
    serial text,
    receipt uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, brand, category, serial)) STORED
);
CREATE INDEX possession_search ON possession USING gin (search);
ALTER TABLE possession ENABLE ROW LEVEL SECURITY;
ALTER TABLE possession FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON possession FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON possession USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE possession IS 'icon: package';
