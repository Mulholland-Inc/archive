SELECT pg_advisory_xact_lock(7271);

CREATE TABLE IF NOT EXISTS schema_version (
    version    int PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS types (
    name       text PRIMARY KEY,
    extends    text NOT NULL DEFAULT '',
    def        jsonb NOT NULL,
    builtin    bool NOT NULL DEFAULT false,
    updated_at timestamptz
);

CREATE TABLE IF NOT EXISTS changes (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    proposed_by  text NOT NULL,
    status       text NOT NULL,
    ops          jsonb NOT NULL,
    from_version int NOT NULL,
    to_version   int NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS spaces (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL UNIQUE,
    role       text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    role       text NOT NULL UNIQUE,
    home       uuid REFERENCES spaces,
    signin     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    kind       text NOT NULL,
    name       text NOT NULL DEFAULT '',
    role       text UNIQUE,
    hash       text NOT NULL UNIQUE,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS tokens_user_idx ON tokens (user_id);

CREATE TABLE IF NOT EXISTS raw (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    source      text NOT NULL,
    external_id text NOT NULL,
    hash        text NOT NULL,
    fetched_at  timestamptz NOT NULL DEFAULT now(),
    payload     jsonb NOT NULL,
    space       uuid NOT NULL REFERENCES spaces
);
ALTER TABLE raw ADD COLUMN IF NOT EXISTS space uuid REFERENCES spaces;
ALTER TABLE raw DROP CONSTRAINT IF EXISTS raw_source_external_id_hash_key;
CREATE UNIQUE INDEX IF NOT EXISTS raw_space_source_key ON raw (space, source, external_id, hash);

CREATE TABLE IF NOT EXISTS records (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    type         text NOT NULL,
    spec_version int NOT NULL,
    data         jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    space        uuid NOT NULL REFERENCES spaces
);
ALTER TABLE records ADD COLUMN IF NOT EXISTS space uuid REFERENCES spaces;
CREATE INDEX IF NOT EXISTS records_data_idx ON records USING gin (data);
CREATE INDEX IF NOT EXISTS records_type_id_idx ON records (type, id);
CREATE INDEX IF NOT EXISTS records_space_idx ON records (space);

CREATE TABLE IF NOT EXISTS provenance (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    record_id  uuid NOT NULL REFERENCES records ON DELETE CASCADE,
    field_path text NOT NULL,
    raw_ids    uuid[] NOT NULL DEFAULT '{}',
    method     text NOT NULL,
    confidence double precision NOT NULL,
    original   text NOT NULL DEFAULT '',
    space      uuid NOT NULL REFERENCES spaces,
    file_id    uuid,
    page       int NOT NULL DEFAULT 0
);
ALTER TABLE provenance ADD COLUMN IF NOT EXISTS space uuid REFERENCES spaces;
ALTER TABLE provenance ADD COLUMN IF NOT EXISTS file_id uuid;
ALTER TABLE provenance ADD COLUMN IF NOT EXISTS page int NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS provenance_record_idx ON provenance (record_id);
CREATE INDEX IF NOT EXISTS provenance_space_idx ON provenance (space);

CREATE TABLE IF NOT EXISTS links (
    id      uuid PRIMARY KEY DEFAULT uuidv7(),
    from_id uuid NOT NULL REFERENCES records ON DELETE CASCADE,
    to_id   uuid NOT NULL REFERENCES records ON DELETE CASCADE,
    kind    text NOT NULL,
    space   uuid NOT NULL REFERENCES spaces
);
ALTER TABLE links ADD COLUMN IF NOT EXISTS space uuid REFERENCES spaces;
CREATE INDEX IF NOT EXISTS links_from_idx ON links (from_id);
CREATE INDEX IF NOT EXISTS links_to_idx ON links (to_id);
CREATE INDEX IF NOT EXISTS links_space_idx ON links (space);

CREATE TABLE IF NOT EXISTS files (
    id         uuid PRIMARY KEY REFERENCES records ON DELETE CASCADE,
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
    space      uuid NOT NULL REFERENCES spaces,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (space, hash, name)
);
CREATE INDEX IF NOT EXISTS files_pending_idx ON files (id) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS sources (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    kind        text NOT NULL,
    name        text NOT NULL,
    space       uuid NOT NULL REFERENCES spaces,
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
CREATE INDEX IF NOT EXISTS sources_next_run_idx ON sources (next_run_at);

CREATE TABLE IF NOT EXISTS views (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL,
    space      uuid NOT NULL REFERENCES spaces,
    type       text NOT NULL,
    layout     text NOT NULL DEFAULT 'table',
    by_field   text NOT NULL DEFAULT '',
    filter     jsonb NOT NULL DEFAULT '{}',
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (space, name)
);

CREATE TABLE IF NOT EXISTS secrets (
    id        uuid PRIMARY KEY,
    record_id uuid REFERENCES records ON DELETE CASCADE,
    source_id uuid REFERENCES sources ON DELETE CASCADE,
    type      text NOT NULL,
    value     bytea NOT NULL,
    space     uuid NOT NULL REFERENCES spaces
);
ALTER TABLE secrets ALTER COLUMN record_id DROP NOT NULL;
ALTER TABLE secrets ADD COLUMN IF NOT EXISTS source_id uuid REFERENCES sources ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS secrets_record_idx ON secrets (record_id);
CREATE INDEX IF NOT EXISTS secrets_source_idx ON secrets (source_id);
CREATE INDEX IF NOT EXISTS secrets_space_idx ON secrets (space);

CREATE TABLE IF NOT EXISTS proposals (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    raw_id     uuid REFERENCES raw ON DELETE CASCADE,
    type       text NOT NULL,
    source     text NOT NULL DEFAULT '',
    label      text NOT NULL DEFAULT '',
    file_id    uuid,
    method     text NOT NULL DEFAULT '',
    fields     jsonb NOT NULL,
    confidence double precision NOT NULL,
    status     text NOT NULL,
    record_id  uuid REFERENCES records ON DELETE SET NULL,
    decided_by text NOT NULL DEFAULT '',
    decided_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    space      uuid NOT NULL REFERENCES spaces
);
CREATE INDEX IF NOT EXISTS proposals_status_idx ON proposals (status, id);
CREATE INDEX IF NOT EXISTS proposals_space_idx ON proposals (space);

DO $$ BEGIN
    IF to_regclass('specs') IS NOT NULL THEN
        INSERT INTO types (name, extends, def, updated_at)
        SELECT t.key, coalesce(t.value->>'extends', ''), t.value - 'extends', now()
        FROM (SELECT spec FROM specs ORDER BY version DESC LIMIT 1) s, jsonb_each(s.spec->'types') t;
        DROP TABLE specs;
    END IF;
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
    SELECT coalesce(array_agg(id), '{}') FROM spaces WHERE pg_has_role(current_user, (role || '_' || lvl)::name, 'USAGE')
$$;

-- Whether role r is an API token of a user who is the instance admin. Definer rights, because user
-- roles cannot read tokens or users; it takes the caller's role as an argument for the same reason.
CREATE OR REPLACE FUNCTION admin_token(r name) RETURNS bool LANGUAGE sql STABLE SECURITY DEFINER SET search_path FROM CURRENT AS $$
    SELECT EXISTS (SELECT FROM tokens t JOIN users u ON u.id = t.user_id WHERE t.role = r::text AND pg_has_role(u.role::name, '{p}_admin', 'USAGE'))
$$;

-- The instance admin, signed in or through a full token, or through one of their API tokens that
-- holds admin in some space. Admin of a space alone is not enough: every user owns their home space,
-- and what this guards (the ontology, sources that run programs or read the server's disk) reaches
-- every space and the server itself.
CREATE OR REPLACE FUNCTION ontology_admin() RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT pg_has_role(current_user, '{p}_admin', 'USAGE') OR (cardinality(my_spaces('admin')) > 0 AND admin_token(current_user))
$$;

CREATE OR REPLACE FUNCTION secret_granted(typ text) RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_secret_' || typ AND pg_has_role(current_user, oid, 'USAGE'))
$$;

CREATE OR REPLACE FUNCTION secrets_prune() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
    DELETE FROM secrets WHERE record_id = NEW.id
        AND id::text NOT IN (SELECT v #>> '{}' FROM jsonb_path_query(NEW.data, 'strict $.**."$secret"') v);
    RETURN NULL;
END $$;
CREATE OR REPLACE TRIGGER records_secrets_prune AFTER UPDATE OF data ON records FOR EACH ROW EXECUTE FUNCTION secrets_prune();

GRANT SELECT ON types, changes, spaces, schema_version TO {p}_user;
GRANT INSERT, UPDATE, DELETE ON types TO {p}_user;
GRANT INSERT, UPDATE ON changes TO {p}_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON sources, files, views, proposals TO {p}_user;

ALTER TABLE types ENABLE ROW LEVEL SECURITY, FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS everyone ON types;
CREATE POLICY everyone ON types FOR SELECT USING (true);
DROP POLICY IF EXISTS admin ON types;
CREATE POLICY admin ON types FOR ALL USING (ontology_admin()) WITH CHECK (ontology_admin());

ALTER TABLE changes ENABLE ROW LEVEL SECURITY, FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS everyone ON changes;
CREATE POLICY everyone ON changes FOR SELECT USING (true);
DROP POLICY IF EXISTS propose ON changes;
CREATE POLICY propose ON changes FOR INSERT WITH CHECK (status = 'proposed' OR ontology_admin());
DROP POLICY IF EXISTS admin ON changes;
CREATE POLICY admin ON changes FOR UPDATE USING (ontology_admin()) WITH CHECK (ontology_admin());

ALTER TABLE spaces ENABLE ROW LEVEL SECURITY, FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS member ON spaces;
CREATE POLICY member ON spaces FOR SELECT USING (pg_has_role(current_user, (role || '_viewer')::name, 'USAGE'));

DO $$ DECLARE t text; BEGIN
    FOREACH t IN ARRAY ARRAY['records', 'raw', 'provenance', 'links', 'secrets', 'sources', 'files', 'views', 'proposals'] LOOP
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

CREATE POLICY viewer ON records FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON raw FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON provenance FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON links FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[])
    AND EXISTS (SELECT FROM records r WHERE r.id = from_id) AND EXISTS (SELECT FROM records r WHERE r.id = to_id));
CREATE POLICY viewer ON files FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON sources FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON views FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON proposals FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
CREATE POLICY viewer ON secrets FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]) AND secret_granted(type));

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

CREATE OR REPLACE FUNCTION search_record(typ text, d jsonb) RETURNS tsvector LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path FROM CURRENT AS $$
    SELECT setweight(to_tsvector('simple', search_fold(typ)), 'A') || setweight(to_tsvector('simple', search_fold(left(search_text(d), 250000))), 'B')
$$;

CREATE OR REPLACE FUNCTION search_file(t text) RETURNS tsvector LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path FROM CURRENT AS $$
    SELECT setweight(to_tsvector('simple', search_fold(left(t, 250000))), 'C')
$$;

CREATE OR REPLACE FUNCTION search_raw(p jsonb) RETURNS tsvector LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path FROM CURRENT AS $$
    SELECT setweight(to_tsvector('simple', search_fold(left(p ->> 'text', 250000))), 'C')
$$;

CREATE OR REPLACE FUNCTION search_query(q text) RETURNS tsquery LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path FROM CURRENT AS $$
    SELECT array_to_string(array(SELECT quote_literal(l) || ':*' FROM unnest(to_tsvector('simple', search_fold(q))) u(l)), ' | ')::tsquery
$$;

CREATE INDEX IF NOT EXISTS records_search_idx ON records USING gin (search_record(type, data));
CREATE INDEX IF NOT EXISTS files_search_idx ON files USING gin (search_file(text));
CREATE INDEX IF NOT EXISTS raw_search_idx ON raw USING gin (search_raw(payload)) WHERE source <> 'file';
CREATE INDEX IF NOT EXISTS provenance_raw_idx ON provenance USING gin (raw_ids);

CREATE TABLE IF NOT EXISTS embed_queue (
    record_id uuid PRIMARY KEY REFERENCES records ON DELETE CASCADE,
    queued_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    retry_at  timestamptz NOT NULL DEFAULT '-infinity',
    tries     int NOT NULL DEFAULT 0,
    error     text NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS embed_queue_at_idx ON embed_queue (queued_at);

CREATE OR REPLACE FUNCTION embed_enqueue() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path FROM CURRENT AS $$
BEGIN
    INSERT INTO embed_queue (record_id) VALUES (NEW.id)
        ON CONFLICT (record_id) DO UPDATE SET queued_at = clock_timestamp(), retry_at = '-infinity', tries = 0, error = '';
    RETURN NULL;
END $$;
CREATE OR REPLACE TRIGGER records_embed_insert AFTER INSERT ON records FOR EACH ROW EXECUTE FUNCTION embed_enqueue();
CREATE OR REPLACE TRIGGER records_embed_update AFTER UPDATE OF data ON records FOR EACH ROW
    WHEN (OLD.data IS DISTINCT FROM NEW.data) EXECUTE FUNCTION embed_enqueue();
CREATE OR REPLACE TRIGGER files_embed_update AFTER UPDATE OF text ON files FOR EACH ROW
    WHEN (OLD.text IS DISTINCT FROM NEW.text) EXECUTE FUNCTION embed_enqueue();

INSERT INTO schema_version (version) VALUES (2), (3), (4), (5), (6), (7), (8) ON CONFLICT DO NOTHING;
