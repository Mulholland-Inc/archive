CREATE TYPE place_kind AS ENUM ('home', 'restaurant', 'cafe', 'bar', 'shop', 'doctor', 'gym', 'park', 'museum', 'office', 'school', 'hotel', 'other');
CREATE TABLE place (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    kind place_kind,
    address archive.address,
    location archive.geo,
    website archive.url,
    rating smallint CHECK (rating BETWEEN 1 AND 5),
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, notes)) STORED
);
CREATE INDEX place_search ON place USING gin (search);
ALTER TABLE place ENABLE ROW LEVEL SECURITY;
ALTER TABLE place FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON place FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON place USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE place IS 'A place you go to or want to remember';
COMMENT ON COLUMN place.name IS 'Name of the place';
COMMENT ON COLUMN place.kind IS 'What sort of place';
COMMENT ON COLUMN place.address IS 'Street address';
COMMENT ON COLUMN place.location IS 'Coordinates';
COMMENT ON COLUMN place.website IS 'Website';
COMMENT ON COLUMN place.rating IS 'Your rating, 1 to 5';
COMMENT ON COLUMN place.notes IS 'What to order, who to ask for, opening quirks';
