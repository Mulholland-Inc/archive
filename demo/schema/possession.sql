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
COMMENT ON TABLE possession IS 'Something you own that is worth keeping track of';
COMMENT ON COLUMN possession.name IS 'What it is';
COMMENT ON COLUMN possession.brand IS 'Brand and model';
COMMENT ON COLUMN possession.category IS 'Like electronics, furniture or bike';
COMMENT ON COLUMN possession.purchased IS 'Purchase date';
COMMENT ON COLUMN possession.price IS 'What you paid';
COMMENT ON COLUMN possession.warranty_until IS 'End of the warranty';
COMMENT ON COLUMN possession.serial IS 'Serial number';
COMMENT ON COLUMN possession.receipt IS 'Receipt or invoice';
