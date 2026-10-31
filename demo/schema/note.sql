CREATE TABLE note (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
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
CREATE POLICY access ON note USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE note IS 'A note, idea or piece of reference
icon: sticky-note';
COMMENT ON COLUMN note.title IS 'Title';
COMMENT ON COLUMN note.body IS 'The note, in Markdown';
COMMENT ON COLUMN note.tags IS 'Tags';
COMMENT ON COLUMN note.pinned IS 'Pinned to the top';
