CREATE TABLE note (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    body archive.richtext,
    tags text[],
    pinned boolean NOT NULL DEFAULT false,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, body)) STORED
);
CREATE INDEX note_search ON note USING gin (search);
ALTER TABLE note ENABLE ROW LEVEL SECURITY;
ALTER TABLE note FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON note FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON note USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE note IS 'icon: sticky-note';
