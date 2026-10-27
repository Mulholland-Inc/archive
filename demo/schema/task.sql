CREATE TYPE task_status AS ENUM ('todo', 'doing', 'waiting', 'done');
CREATE TYPE task_priority AS ENUM ('low', 'normal', 'high', 'urgent');
CREATE TYPE life_area AS ENUM ('home', 'health', 'money', 'admin', 'work', 'social', 'learning', 'travel');
CREATE TABLE task (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title text NOT NULL,
    status task_status NOT NULL DEFAULT 'todo',
    priority task_priority,
    area life_area,
    due date,
    person uuid REFERENCES person,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(title, notes)) STORED
);
CREATE INDEX task_search ON task USING gin (search);
ALTER TABLE task ENABLE ROW LEVEL SECURITY;
ALTER TABLE task FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON task FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON task USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE task IS 'icon: square-check';
