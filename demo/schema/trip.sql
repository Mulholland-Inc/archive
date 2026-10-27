CREATE TABLE trip (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    destination text,
    country archive.country,
    starts date,
    ends date,
    budget archive.money,
    people uuid[],
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, destination, notes)) STORED
);
CREATE INDEX trip_search ON trip USING gin (search);
ALTER TABLE trip ENABLE ROW LEVEL SECURITY;
ALTER TABLE trip FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON trip FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON trip USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE trip IS 'A trip or holiday, past or planned';
COMMENT ON COLUMN trip.name IS 'Name of the trip';
COMMENT ON COLUMN trip.destination IS 'City or region';
COMMENT ON COLUMN trip.country IS 'Country';
COMMENT ON COLUMN trip.starts IS 'First day';
COMMENT ON COLUMN trip.ends IS 'Last day';
COMMENT ON COLUMN trip.budget IS 'Budget';
COMMENT ON COLUMN trip.people IS 'references person';
COMMENT ON COLUMN trip.notes IS 'Plans, bookings, what to pack';
