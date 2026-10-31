CREATE TABLE journal (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    day date NOT NULL,
    mood smallint CHECK (mood BETWEEN 1 AND 5),
    entry text NOT NULL,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(entry)) STORED
);
CREATE INDEX journal_search ON journal USING gin (search);
ALTER TABLE journal ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON journal USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE journal IS 'A diary entry for a day
icon: notebook-pen';
COMMENT ON COLUMN journal.day IS 'The day';
COMMENT ON COLUMN journal.mood IS 'Mood, 1 (bad) to 5 (great)';
COMMENT ON COLUMN journal.entry IS 'What happened and how it felt';
