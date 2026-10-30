CREATE TYPE parent_link_kind AS ENUM ('birth', 'adopted', 'step', 'foster');
CREATE TABLE parent_link (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
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
CREATE POLICY viewer ON parent_link FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON parent_link USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE parent_link IS 'A child of a partnership
icon: git-fork';
COMMENT ON COLUMN parent_link.child IS 'The child';
COMMENT ON COLUMN parent_link.partnership IS 'The parents';
COMMENT ON COLUMN parent_link.kind IS 'Birth, adopted, step or foster';
COMMENT ON COLUMN parent_link.notes IS 'Anything else worth keeping';
