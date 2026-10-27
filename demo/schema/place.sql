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
COMMENT ON TABLE place IS 'icon: map-pin';
