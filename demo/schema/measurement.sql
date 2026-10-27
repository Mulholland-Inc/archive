CREATE TYPE measurement_kind AS ENUM ('weight', 'sleep', 'steps', 'resting_heart_rate', 'blood_pressure_systolic', 'blood_pressure_diastolic', 'run_distance');
CREATE TABLE measurement (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    taken date NOT NULL,
    kind measurement_kind NOT NULL,
    value double precision NOT NULL,
    unit text NOT NULL,
    notes text,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(notes)) STORED
);
CREATE INDEX measurement_search ON measurement USING gin (search);
ALTER TABLE measurement ENABLE ROW LEVEL SECURITY;
ALTER TABLE measurement FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON measurement FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON measurement USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE measurement IS 'icon: activity';
