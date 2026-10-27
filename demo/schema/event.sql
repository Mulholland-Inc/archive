CREATE TABLE event (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    starts timestamptz NOT NULL,
    ends timestamptz,
    place uuid REFERENCES place,
    people uuid[],
    recurrence archive.recurrence,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, notes)) STORED
);
CREATE INDEX event_search ON event USING gin (search);
ALTER TABLE event ENABLE ROW LEVEL SECURITY;
ALTER TABLE event FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON event FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON event USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE event IS 'Something on your calendar
icon: calendar-days';
COMMENT ON COLUMN event.title IS 'What it is';
COMMENT ON COLUMN event.starts IS 'Start time';
COMMENT ON COLUMN event.ends IS 'End time';
COMMENT ON COLUMN event.place IS 'Where';
COMMENT ON COLUMN event.people IS 'references person';
COMMENT ON COLUMN event.recurrence IS 'Repeat rule, if it repeats';
COMMENT ON COLUMN event.notes IS 'Agenda or things to bring';
