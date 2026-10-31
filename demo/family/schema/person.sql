CREATE TYPE person_sex AS ENUM ('m', 'f', 'u');
CREATE TABLE person (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    sex person_sex NOT NULL DEFAULT 'u',
    born text,
    born_year integer,
    died text,
    died_year integer,
    birthplace text,
    home boolean NOT NULL DEFAULT false,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, notes)) STORED
);
CREATE INDEX person_search ON person USING gin (search);
ALTER TABLE person ENABLE ROW LEVEL SECURITY;
ALTER TABLE person FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON person FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON person USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE person IS 'Someone in the family
icon: user';
COMMENT ON COLUMN person.name IS 'Full name';
COMMENT ON COLUMN person.sex IS 'As recorded';
COMMENT ON COLUMN person.born IS 'Birth date as written, like about 1890';
COMMENT ON COLUMN person.born_year IS 'Birth year, for sorting';
COMMENT ON COLUMN person.died IS 'Death date as written';
COMMENT ON COLUMN person.died_year IS 'Death year, for sorting';
COMMENT ON COLUMN person.birthplace IS 'Where they were born';
COMMENT ON COLUMN person.home IS 'The person the tree opens on';
COMMENT ON COLUMN person.notes IS 'Anything else worth keeping';
