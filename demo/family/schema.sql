CREATE TYPE person_sex AS ENUM ('m', 'f', 'u');
CREATE TABLE person (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
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
CREATE POLICY access ON person USING (archive.can(access)) WITH CHECK (archive.can(access));
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

CREATE TABLE partnership (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
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
CREATE POLICY access ON partnership USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE partnership IS 'Two people who married or had children together
icon: heart';
COMMENT ON COLUMN partnership.a IS 'One partner, drawn on the left';
COMMENT ON COLUMN partnership.b IS 'The other partner, if known';
COMMENT ON COLUMN partnership.notes IS 'Anything else worth keeping';

CREATE TYPE parent_link_kind AS ENUM ('birth', 'adopted', 'step', 'foster');
CREATE TABLE parent_link (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    child uuid NOT NULL REFERENCES person,
    partnership uuid NOT NULL REFERENCES partnership,
    kind parent_link_kind NOT NULL DEFAULT 'birth',
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(notes)) STORED
);
CREATE INDEX parent_link_search ON parent_link USING gin (search);
ALTER TABLE parent_link ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_link FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON parent_link USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE parent_link IS 'A child of a partnership
icon: git-fork';
COMMENT ON COLUMN parent_link.child IS 'The child';
COMMENT ON COLUMN parent_link.partnership IS 'The parents';
COMMENT ON COLUMN parent_link.kind IS 'Birth, adopted, step or foster';
COMMENT ON COLUMN parent_link.notes IS 'Anything else worth keeping';

CREATE VIEW family_tree WITH (security_invoker = true) AS
SELECT p.*,
    p.died IS NULL AND p.died_year IS NULL AND (p.born_year IS NULL OR p.born_year > extract(year FROM now()) - 110) AS living,
    ARRAY(SELECT l.partnership FROM parent_link l WHERE l.child = p.id ORDER BY l.created_at) AS parents
FROM person p;
COMMENT ON VIEW family_tree IS 'The family, oldest at the top
icon: network';

