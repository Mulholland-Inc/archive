SELECT pg_advisory_xact_lock(7271);

CREATE TABLE IF NOT EXISTS type (
    name       text PRIMARY KEY,
    extends    text NOT NULL DEFAULT '',
    def        jsonb NOT NULL,
    builtin    bool NOT NULL DEFAULT false,
    updated_at timestamptz
);

CREATE TABLE IF NOT EXISTS space (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL UNIQUE,
    role       text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS "user" (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    role       text NOT NULL UNIQUE,
    home       uuid REFERENCES space,
    signin     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS token (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES "user" ON DELETE CASCADE,
    kind       text NOT NULL,
    name       text NOT NULL DEFAULT '',
    role       text UNIQUE,
    hash       text NOT NULL UNIQUE,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS token_user_idx ON token (user_id);

CREATE TABLE IF NOT EXISTS entity (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    type       text NOT NULL,
    data       jsonb NOT NULL,
    proposed   bool NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    space      uuid NOT NULL REFERENCES space
);
CREATE INDEX IF NOT EXISTS entity_data_idx ON entity USING gin (data);
CREATE INDEX IF NOT EXISTS entity_type_id_idx ON entity (type, id);
CREATE INDEX IF NOT EXISTS entity_space_idx ON entity (space);
CREATE INDEX IF NOT EXISTS entity_proposed_idx ON entity (id) WHERE proposed;

CREATE TABLE IF NOT EXISTS file (
    id         uuid PRIMARY KEY REFERENCES entity ON DELETE CASCADE,
    hash       text NOT NULL,
    name       text NOT NULL,
    mime       text NOT NULL,
    size       bigint NOT NULL,
    metadata   jsonb NOT NULL DEFAULT '{}',
    text       text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'pending',
    error      text NOT NULL DEFAULT '',
    intake     text NOT NULL DEFAULT '',
    owner      text NOT NULL,
    space      uuid NOT NULL REFERENCES space,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (space, hash, name)
);
CREATE INDEX IF NOT EXISTS file_pending_idx ON file (id) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS source (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    kind        text NOT NULL,
    name        text NOT NULL,
    space       uuid NOT NULL REFERENCES space,
    owner       text NOT NULL,
    created_by  text NOT NULL DEFAULT '',
    type        text NOT NULL DEFAULT '',
    mode        text NOT NULL DEFAULT 'mirror',
    schedule    text NOT NULL DEFAULT '',
    rate        double precision NOT NULL DEFAULT 5,
    config      jsonb NOT NULL DEFAULT '{}',
    state       jsonb NOT NULL DEFAULT '{}',
    status      text NOT NULL DEFAULT 'new',
    error       text NOT NULL DEFAULT '',
    failures    int NOT NULL DEFAULT 0,
    last_run_at timestamptz,
    next_run_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (space, name)
);
CREATE INDEX IF NOT EXISTS source_next_run_idx ON source (next_run_at);

CREATE TABLE IF NOT EXISTS view (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL,
    space      uuid NOT NULL REFERENCES space,
    type       text NOT NULL,
    layout     text NOT NULL DEFAULT 'table',
    by_field   text NOT NULL DEFAULT '',
    filter     jsonb NOT NULL DEFAULT '{}',
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (space, name)
);

CREATE TABLE IF NOT EXISTS secret (
    id        uuid PRIMARY KEY,
    entity_id uuid REFERENCES entity ON DELETE CASCADE,
    source_id uuid REFERENCES source ON DELETE CASCADE,
    type      text NOT NULL,
    value     bytea NOT NULL,
    space     uuid NOT NULL REFERENCES space
);
CREATE INDEX IF NOT EXISTS secret_entity_idx ON secret (entity_id);
CREATE INDEX IF NOT EXISTS secret_source_idx ON secret (source_id);
CREATE INDEX IF NOT EXISTS secret_space_idx ON secret (space);

DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_authenticator') THEN
        CREATE ROLE {p}_authenticator LOGIN NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_user') THEN
        CREATE ROLE {p}_user NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_admin') THEN
        CREATE ROLE {p}_admin NOLOGIN;
    END IF;
END $$;

CREATE OR REPLACE FUNCTION my_spaces(lvl text DEFAULT 'viewer') RETURNS uuid[] LANGUAGE sql STABLE AS $$
    SELECT coalesce(array_agg(id), '{}') FROM space WHERE pg_has_role(current_user, (role || '_' || lvl)::name, 'USAGE')
$$;

CREATE OR REPLACE FUNCTION ontology_admin() RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT pg_has_role(current_user, '{p}_admin', 'USAGE') OR cardinality(my_spaces('admin')) > 0
$$;

CREATE OR REPLACE FUNCTION secret_granted(typ text) RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_secret_' || typ AND pg_has_role(current_user, oid, 'USAGE'))
$$;

CREATE OR REPLACE FUNCTION secret_prune() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
    DELETE FROM secret WHERE entity_id = NEW.id
        AND id::text NOT IN (SELECT v #>> '{}' FROM jsonb_path_query(NEW.data, 'strict $.**."$secret"') v);
    RETURN NULL;
END $$;
CREATE OR REPLACE TRIGGER entity_secret_prune AFTER UPDATE OF data ON entity FOR EACH ROW EXECUTE FUNCTION secret_prune();

GRANT SELECT ON type, space TO {p}_user;
GRANT INSERT, UPDATE, DELETE ON type TO {p}_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON source, file, view TO {p}_user;

ALTER TABLE type ENABLE ROW LEVEL SECURITY, FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS everyone ON type;
CREATE POLICY everyone ON type FOR SELECT USING (true);
DROP POLICY IF EXISTS admin ON type;
CREATE POLICY admin ON type FOR ALL USING (ontology_admin()) WITH CHECK (ontology_admin());

ALTER TABLE space ENABLE ROW LEVEL SECURITY, FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS member ON space;
CREATE POLICY member ON space FOR SELECT USING (pg_has_role(current_user, (role || '_viewer')::name, 'USAGE'));

DO $$ DECLARE t text; BEGIN
    FOREACH t IN ARRAY ARRAY['entity', 'secret', 'source', 'file', 'view'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY, FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS viewer ON %I', t);
        EXECUTE format('DROP POLICY IF EXISTS editor_insert ON %I', t);
        EXECUTE format('DROP POLICY IF EXISTS editor_update ON %I', t);
        EXECUTE format('DROP POLICY IF EXISTS editor_delete ON %I', t);
        EXECUTE format('CREATE POLICY editor_insert ON %I FOR INSERT WITH CHECK (space = ANY ((SELECT my_spaces(''editor''))::uuid[]))', t);
        EXECUTE format('CREATE POLICY editor_update ON %I FOR UPDATE USING (space = ANY ((SELECT my_spaces(''editor''))::uuid[])) WITH CHECK (space = ANY ((SELECT my_spaces(''editor''))::uuid[]))', t);
        EXECUTE format('CREATE POLICY editor_delete ON %I FOR DELETE USING (space = ANY ((SELECT my_spaces(''editor''))::uuid[]))', t);
    END LOOP;
END $$;

CREATE POLICY viewer ON entity FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON file FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON view FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON source FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON secret FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]) AND secret_granted(type));

CREATE OR REPLACE FUNCTION search_fold(t text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT translate(replace(replace(replace(lower(regexp_replace(regexp_replace(normalize(coalesce(t, ''), NFD),
        '[̀-ͯ]', '', 'g'), '[._/\\]+', ' ', 'g')), 'ß', 'ss'), 'æ', 'ae'), 'œ', 'oe'), 'øłđ', 'old')
$$;

CREATE OR REPLACE FUNCTION search_text(d jsonb) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    WITH RECURSIVE t(v) AS (
        SELECT d
        UNION ALL
        SELECT c.value FROM t, LATERAL (
            SELECT value FROM jsonb_each(CASE WHEN jsonb_typeof(t.v) = 'object' AND NOT t.v ? '$secret' THEN t.v END)
            UNION ALL
            SELECT value FROM jsonb_array_elements(CASE WHEN jsonb_typeof(t.v) = 'array' THEN t.v END)
        ) c
    )
    SELECT coalesce(string_agg(v #>> '{}', ' '), '') FROM t WHERE jsonb_typeof(v) IN ('string', 'number')
$$;

CREATE OR REPLACE FUNCTION search_entity(typ text, d jsonb) RETURNS tsvector LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path FROM CURRENT AS $$
    SELECT setweight(to_tsvector('simple', search_fold(typ)), 'A') || setweight(to_tsvector('simple', search_fold(left(search_text(d), 250000))), 'B')
$$;

CREATE OR REPLACE FUNCTION search_file(t text) RETURNS tsvector LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path FROM CURRENT AS $$
    SELECT setweight(to_tsvector('simple', search_fold(left(t, 250000))), 'C')
$$;

CREATE OR REPLACE FUNCTION search_query(q text) RETURNS tsquery LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path FROM CURRENT AS $$
    SELECT array_to_string(array(SELECT quote_literal(l) || ':*' FROM unnest(to_tsvector('simple', search_fold(q))) u(l)), ' | ')::tsquery
$$;

CREATE INDEX IF NOT EXISTS entity_search_idx ON entity USING gin (search_entity(type, data));
CREATE INDEX IF NOT EXISTS file_search_idx ON file USING gin (search_file(text));
