CREATE TYPE person_relation AS ENUM ('partner', 'family', 'friend', 'colleague', 'neighbour', 'doctor', 'service', 'other');
CREATE TABLE person (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    relation person_relation,
    birthday date,
    email archive.email,
    phone archive.phone,
    address archive.address,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, notes)) STORED
);
CREATE INDEX person_search ON person USING gin (search);
ALTER TABLE person ENABLE ROW LEVEL SECURITY;
ALTER TABLE person FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON person FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON person USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE person IS 'Someone in your life: family, friends, colleagues, doctors
icon: user';
COMMENT ON COLUMN person.name IS 'Full name';
COMMENT ON COLUMN person.relation IS 'How you know them';
COMMENT ON COLUMN person.birthday IS 'Date of birth';
COMMENT ON COLUMN person.email IS 'Personal email';
COMMENT ON COLUMN person.phone IS 'Mobile number';
COMMENT ON COLUMN person.address IS 'Home address';
COMMENT ON COLUMN person.notes IS 'Things to remember about them';
