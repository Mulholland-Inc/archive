CREATE TABLE recipe (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
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
CREATE POLICY access ON recipe USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE recipe IS 'A recipe you cook
icon: chef-hat';
COMMENT ON COLUMN recipe.name IS 'Dish';
COMMENT ON COLUMN recipe.cuisine IS 'Cuisine';
COMMENT ON COLUMN recipe.minutes IS 'Total time in minutes';
COMMENT ON COLUMN recipe.servings IS 'Servings';
COMMENT ON COLUMN recipe.ingredients IS 'Ingredients with amounts, one per item';
COMMENT ON COLUMN recipe.steps IS 'Method';
COMMENT ON COLUMN recipe.source IS 'Where it came from';
COMMENT ON COLUMN recipe.tags IS 'Tags like vegetarian or quick';
