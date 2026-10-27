CREATE TABLE recipe (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    name text NOT NULL,
    cuisine text,
    minutes integer,
    servings integer,
    ingredients text[],
    steps text,
    source archive.url,
    tags text[],
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, cuisine, steps)) STORED
);
CREATE INDEX recipe_search ON recipe USING gin (search);
ALTER TABLE recipe ENABLE ROW LEVEL SECURITY;
ALTER TABLE recipe FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON recipe FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON recipe USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
COMMENT ON TABLE recipe IS 'icon: chef-hat';
