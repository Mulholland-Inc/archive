CREATE SCHEMA archive;

CREATE TABLE archive.workspace (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL,
    key        bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive."user" (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    workspaces uuid[] NOT NULL DEFAULT '{}',
    signin     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_workspaces_idx ON archive."user" USING gin (workspaces);

CREATE TABLE archive.token (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES archive."user" ON DELETE CASCADE,
    workspace  uuid REFERENCES archive.workspace ON DELETE CASCADE,
    kind       text NOT NULL,
    name       text NOT NULL DEFAULT '',
    role       text UNIQUE,
    hash       text NOT NULL UNIQUE,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX token_user_idx ON archive.token (user_id);

CREATE TABLE archive.chat (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL,
    title      text NOT NULL DEFAULT '',
    messages   jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_user_idx ON archive.chat (user_id, updated_at);

CREATE TABLE archive.workflow (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL,
    role       text NOT NULL,
    name       text NOT NULL,
    source     text NOT NULL,
    trigger    text NOT NULL DEFAULT '',
    hook       text NOT NULL UNIQUE,
    next_at    timestamptz,
    cursor_at  timestamptz,
    cursor_id  uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);
CREATE INDEX workflow_next_idx ON archive.workflow (next_at) WHERE next_at IS NOT NULL;

CREATE TABLE archive.workflow_run (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    workflow   uuid NOT NULL REFERENCES archive.workflow ON DELETE CASCADE,
    trigger    text NOT NULL,
    result     jsonb NOT NULL DEFAULT 'null',
    logs       jsonb NOT NULL DEFAULT '[]',
    error      text NOT NULL DEFAULT '',
    ms         integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX workflow_run_idx ON archive.workflow_run (workflow, created_at);

CREATE TABLE archive.revision (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    hash       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    author     text NOT NULL DEFAULT '',
    name       text UNIQUE,
    statements jsonb NOT NULL DEFAULT '[]'
);

CREATE TABLE archive.ddl_key (
    key        text PRIMARY KEY,
    pid        integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.due (
    workspace uuid NOT NULL REFERENCES archive.workspace ON DELETE CASCADE,
    kind      text NOT NULL,
    at        timestamptz NOT NULL,
    PRIMARY KEY (workspace, kind)
);

CREATE FUNCTION archive.search_fold(t text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT translate(replace(replace(replace(lower(regexp_replace(regexp_replace(normalize(coalesce(t, ''), NFD),
        '[̀-ͯ]', '', 'g'), '[._/\\]+', ' ', 'g')), 'ß', 'ss'), 'æ', 'ae'), 'œ', 'oe'), 'øłđ', 'old')
$$;

CREATE FUNCTION archive.search_doc(VARIADIC parts text[]) RETURNS tsvector LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT to_tsvector('simple', archive.search_fold(left(array_to_string(parts, ' '), 250000)))
$$;

CREATE FUNCTION archive.search_query(q text) RETURNS tsquery LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT array_to_string(array(SELECT quote_literal(l) || ':*' FROM unnest(to_tsvector('simple', archive.search_fold(q))) u(l)), ' | ')::tsquery
$$;

CREATE TABLE archive.file (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    hash       text NOT NULL,
    name       text NOT NULL,
    mime       text NOT NULL,
    size       bigint NOT NULL,
    metadata   jsonb NOT NULL DEFAULT '{}',
    text       text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'pending',
    error      text NOT NULL DEFAULT '',
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    search     tsvector GENERATED ALWAYS AS (setweight(archive.search_doc(name), 'A') || setweight(archive.search_doc(text), 'C')) STORED,
    UNIQUE NULLS NOT DISTINCT (access, hash, name)
);
CREATE INDEX file_pending_idx ON archive.file (id) WHERE status = 'pending';
CREATE INDEX file_search_idx ON archive.file USING gin (search);

CREATE TABLE archive.upload (
    id         uuid PRIMARY KEY,
    role       text NOT NULL,
    access     text,
    name       text NOT NULL,
    length     bigint NOT NULL,
    parts      bigint[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.secret (
    id          uuid PRIMARY KEY,
    table_name  text NOT NULL,
    row_id      uuid NOT NULL,
    column_name text NOT NULL,
    type        text NOT NULL,
    value       bytea NOT NULL,
    access      text,
    UNIQUE (table_name, row_id, column_name)
);

CREATE DOMAIN archive.duration AS text CHECK (VALUE ~ '^P([0-9]+Y)?([0-9]+M)?([0-9]+D)?(T([0-9]+H)?([0-9]+M)?([0-9]+(\.[0-9]+)?S)?)?$');
COMMENT ON DOMAIN archive.duration IS 'ISO 8601 duration';
CREATE DOMAIN archive.timezone AS text CHECK (VALUE ~ '^[A-Za-z][A-Za-z0-9_+\-/]*$');
COMMENT ON DOMAIN archive.timezone IS 'IANA time zone';
CREATE DOMAIN archive.recurrence AS text CHECK (VALUE ~ '^FREQ=(SECONDLY|MINUTELY|HOURLY|DAILY|WEEKLY|MONTHLY|YEARLY)(;[A-Z]+=[A-Z0-9,+\-]+)*$');
COMMENT ON DOMAIN archive.recurrence IS 'RFC 5545 recurrence rule';
CREATE DOMAIN archive.currency AS text CHECK (VALUE ~ '^[A-Z]{3}$');
COMMENT ON DOMAIN archive.currency IS 'ISO 4217 currency code';
CREATE DOMAIN archive.percentage AS numeric;
COMMENT ON DOMAIN archive.percentage IS 'Fraction, 0.21 is 21%';
CREATE DOMAIN archive.bic AS text CHECK (VALUE ~ '^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$');
COMMENT ON DOMAIN archive.bic IS 'ISO 9362 bank identifier';
CREATE DOMAIN archive.card AS text;
COMMENT ON DOMAIN archive.card IS 'Payment card, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.us_routing AS text CHECK (VALUE ~ '^[0-9]{9}$');
COMMENT ON DOMAIN archive.us_routing IS 'US ABA routing number';
CREATE DOMAIN archive.lei AS text CHECK (VALUE ~ '^[A-Z0-9]{18}[0-9]{2}$');
COMMENT ON DOMAIN archive.lei IS 'ISO 17442 legal entity identifier';
CREATE DOMAIN archive.isin AS text CHECK (VALUE ~ '^[A-Z]{2}[A-Z0-9]{9}[0-9]$');
COMMENT ON DOMAIN archive.isin IS 'ISO 6166 securities identifier';
CREATE DOMAIN archive.us_ein AS text CHECK (VALUE ~ '^(0[1-6]|1[0-6]|2[0-7]|3[0-9]|4[0-8]|5[0-9]|6[0-8]|7[1-7]|8[0-8]|9[0-58-9])-[0-9]{7}$');
COMMENT ON DOMAIN archive.us_ein IS 'US employer identification number';
CREATE DOMAIN archive.us_ssn AS text;
COMMENT ON DOMAIN archive.us_ssn IS 'US social security number, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.national AS text;
COMMENT ON DOMAIN archive.national IS 'National id number with its country and scheme, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.passport AS text;
COMMENT ON DOMAIN archive.passport IS 'Passport number with its issuing country, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.email AS text CHECK (VALUE ~ '^[^@\s]+@[^@\s]+$');
COMMENT ON DOMAIN archive.email IS 'Email address; the domain is lowercase and punycode';
CREATE DOMAIN archive.phone AS text CHECK (VALUE ~ '^\+[1-9][0-9]{6,14}$');
COMMENT ON DOMAIN archive.phone IS 'Phone number in E.164';
CREATE DOMAIN archive.url AS text;
COMMENT ON DOMAIN archive.url IS 'Web address, WHATWG-normalized';
CREATE DOMAIN archive.country AS text CHECK (VALUE ~ '^[A-Z]{2}$');
COMMENT ON DOMAIN archive.country IS 'ISO 3166-1 alpha-2 country code';
CREATE DOMAIN archive.region AS text CHECK (VALUE ~ '^[A-Z]{2}-[A-Z0-9]{1,3}$');
COMMENT ON DOMAIN archive.region IS 'ISO 3166-2 subdivision code';
CREATE DOMAIN archive.locale AS text CHECK (VALUE ~ '^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$');
COMMENT ON DOMAIN archive.locale IS 'BCP 47 language tag';
CREATE DOMAIN archive.gtin AS text CHECK (VALUE ~ '^[0-9]{14}$');
COMMENT ON DOMAIN archive.gtin IS 'GS1 trade item number as 14 digits; accepts EAN-13 and UPC-A';
CREATE DOMAIN archive.isbn AS text CHECK (VALUE ~ '^97[89][0-9]{10}$');
COMMENT ON DOMAIN archive.isbn IS 'ISBN-13; ISBN-10 is converted';
CREATE DOMAIN archive.ip AS inet;
COMMENT ON DOMAIN archive.ip IS 'IPv4 or IPv6 address';
CREATE DOMAIN archive.domain AS text CHECK (VALUE ~ '^[a-z0-9]([a-z0-9.\-]*[a-z0-9])?$');
COMMENT ON DOMAIN archive.domain IS 'Domain name, lowercase punycode';
CREATE DOMAIN archive.mime AS text CHECK (VALUE ~ '^[a-z]+/[a-z0-9!#$&^_.+\-]+(;.*)?$');
COMMENT ON DOMAIN archive.mime IS 'IANA media type';
CREATE DOMAIN archive.richtext AS text;
COMMENT ON DOMAIN archive.richtext IS 'Markdown (CommonMark)';

CREATE TYPE archive.timestamp AS (utc timestamptz, zone archive.timezone);
COMMENT ON TYPE archive.timestamp IS 'Instant in UTC, with the IANA zone it was given in';
CREATE TYPE archive.time AS (time time, zone archive.timezone);
COMMENT ON TYPE archive.time IS 'Time of day, with an optional IANA zone';
CREATE TYPE archive.interval AS (start text, "end" text);
COMMENT ON TYPE archive.interval IS 'Start and end, each a date or UTC instant; null is open';
CREATE TYPE archive.money AS (amount numeric, currency archive.currency);
COMMENT ON TYPE archive.money IS 'Amount in the currency minor units, with its currency';
CREATE TYPE archive.address AS (lines text[], locality text, region archive.region, postcode text, country archive.country);
COMMENT ON TYPE archive.address IS 'Postal address';
CREATE TYPE archive.name AS ("full" text, given text, family text, prefixes text);
COMMENT ON TYPE archive.name IS 'Person name: the full original, and its parts when known';
CREATE TYPE archive.postcode AS (country archive.country, code text);
COMMENT ON TYPE archive.postcode IS 'Postcode with the country it is valid in';
CREATE TYPE archive.geo AS (lat double precision, lon double precision);
COMMENT ON TYPE archive.geo IS 'WGS 84 latitude and longitude';
CREATE TYPE archive.quantity AS (value double precision, unit text, original_unit text);
COMMENT ON TYPE archive.quantity IS 'Value in the SI base unit, with the UCUM unit it was given in';

CREATE TABLE archive.composite_use (
    "timestamp" archive.timestamp,
    "time"      archive.time,
    "interval"  archive.interval,
    money       archive.money,
    address     archive.address,
    name        archive.name,
    postcode    archive.postcode,
    geo         archive.geo,
    quantity    archive.quantity
);
COMMENT ON TABLE archive.composite_use IS 'Always empty; it uses every built-in composite type, so pg-schema-diff sees them as used by a table before and after a user table starts using one';

CREATE TYPE archive.level AS ENUM ('viewer', 'editor', 'owner');
COMMENT ON TYPE archive.level IS 'What a member may do: a viewer reads, an editor also writes, an owner also changes the schema and the members and sees every row and secret';

CREATE TABLE archive.member (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      archive.email NOT NULL UNIQUE,
    role       archive.level NOT NULL DEFAULT 'viewer',
    groups     text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE archive.member IS 'A person in this workspace. Owners add, change and remove members; adding an email invites that person
icon: users';
COMMENT ON COLUMN archive.member.email IS 'The address they sign in with; it cannot change';
COMMENT ON COLUMN archive.member.groups IS 'Groups they are in, like hr; a row whose access is hr is seen only by the hr group and the owners';

CREATE FUNCTION archive.can(g text) RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT g IS NULL OR pg_has_role(current_user, '{p}_owner', 'USAGE')
        OR EXISTS (SELECT FROM pg_catalog.pg_roles r WHERE r.rolname = '{p}_g_' || g AND pg_has_role(current_user, r.oid, 'USAGE'))
$$;

CREATE FUNCTION archive.member_check() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.email := lower(btrim(NEW.email));
    IF TG_OP = 'UPDATE' AND NEW.email <> OLD.email THEN
        RAISE EXCEPTION 'an email cannot change; remove the member and add the new address' USING ERRCODE = '22023', TABLE = 'member', COLUMN = 'email';
    END IF;
    NEW.groups := ARRAY(SELECT DISTINCT g FROM unnest(NEW.groups) g WHERE g IS NOT NULL ORDER BY g);
    IF EXISTS (SELECT FROM unnest(NEW.groups) g WHERE g !~ '^[a-z0-9][a-z0-9_]{0,23}$') THEN
        RAISE EXCEPTION 'want group names of 1-24 lowercase letters, digits or _' USING ERRCODE = '22023', TABLE = 'member', COLUMN = 'groups';
    END IF;
    RETURN NEW;
END $$;

CREATE FUNCTION archive.member_roles() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    rid   uuid;
    u     text;
    m     text;
    g     text;
    added text[] := '{}';
    gone  text[] := '{}';
BEGIN
    PERFORM pg_advisory_xact_lock(7273);
    IF TG_OP <> 'INSERT' AND OLD.role = 'owner' AND (TG_OP = 'DELETE' OR NEW.role <> 'owner')
        AND NOT EXISTS (SELECT FROM archive.member x WHERE x.role = 'owner' AND x.id <> OLD.id) THEN
        RAISE EXCEPTION '% is the last owner of this workspace', OLD.email USING ERRCODE = '23001';
    END IF;
    IF TG_OP = 'DELETE' THEN
        rid := OLD.id;
    ELSE
        rid := NEW.id;
    END IF;
    u := '{p}_u' || translate(rtrim(encode(uuid_send(rid), 'base64'), '='), '+/', '-_');
    m := '{p}_m' || translate(rtrim(encode(uuid_send(rid), 'base64'), '='), '+/', '-_');
    IF TG_OP = 'DELETE' THEN
        EXECUTE format('DROP ROLE IF EXISTS %I, %I', u, m);
        gone := OLD.groups;
    ELSE
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = u) THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN', u);
        END IF;
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = m) THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN', m);
        END IF;
        IF TG_OP = 'INSERT' OR NEW.role <> OLD.role THEN
            EXECUTE format('REVOKE %I, %I, %I FROM %I', '{p}_viewer', '{p}_editor', '{p}_owner', u);
            EXECUTE format('GRANT %I TO %I', '{p}_' || NEW.role, u);
        END IF;
        EXECUTE format('GRANT %I TO %I', m, u);
        EXECUTE format('GRANT %I TO %I', u, '{p}_authenticator');
        IF TG_OP = 'INSERT' THEN
            added := NEW.groups;
        ELSE
            added := ARRAY(SELECT unnest(NEW.groups) EXCEPT SELECT unnest(OLD.groups));
            gone := ARRAY(SELECT unnest(OLD.groups) EXCEPT SELECT unnest(NEW.groups));
        END IF;
        FOREACH g IN ARRAY added LOOP
            IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_g_' || g) THEN
                EXECUTE format('CREATE ROLE %I NOLOGIN', '{p}_g_' || g);
            END IF;
            EXECUTE format('GRANT %I TO %I', '{p}_g_' || g, m);
        END LOOP;
        FOREACH g IN ARRAY gone LOOP
            EXECUTE format('REVOKE %I FROM %I', '{p}_g_' || g, m);
        END LOOP;
    END IF;
    FOREACH g IN ARRAY gone LOOP
        IF NOT EXISTS (SELECT FROM archive.member x WHERE g = ANY (x.groups)) THEN
            EXECUTE format('DROP ROLE IF EXISTS %I', '{p}_g_' || g);
        END IF;
    END LOOP;
    IF TG_OP <> 'UPDATE' THEN
        PERFORM set_config('archive.members', '1', true);
    END IF;
    RETURN NULL;
END $$;

CREATE TRIGGER member_check BEFORE INSERT OR UPDATE ON archive.member FOR EACH ROW EXECUTE FUNCTION archive.member_check();
CREATE TRIGGER member_roles AFTER INSERT OR UPDATE OR DELETE ON archive.member FOR EACH ROW EXECUTE FUNCTION archive.member_roles();

CREATE FUNCTION archive.secret_clear(tbl text, rid uuid, col text) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
    DELETE FROM archive.secret s
    WHERE s.table_name = tbl AND s.row_id = rid AND (col IS NULL OR s.column_name = col)
      AND pg_has_role(current_setting('role')::name, '{p}_editor', 'USAGE')
      AND (s.access IS NULL OR pg_has_role(current_setting('role')::name, '{p}_owner', 'USAGE')
        OR EXISTS (SELECT FROM pg_catalog.pg_roles r WHERE r.rolname = '{p}_g_' || s.access AND pg_has_role(current_setting('role')::name, r.oid, 'USAGE')))
$$;

CREATE FUNCTION archive.ddl_allowed(k text) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF NOT EXISTS (SELECT FROM archive.ddl_key d WHERE d.key = k AND d.pid = pg_backend_pid()) THEN
        RAISE EXCEPTION 'only archive.migrate changes the schema' USING ERRCODE = '42501';
    END IF;
END $$;

CREATE FUNCTION archive.ddl(k text, stmt text) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path = public AS $$
BEGIN
    PERFORM archive.ddl_allowed(k);
    EXECUTE stmt;
END $$;

CREATE FUNCTION archive.ddl_revision(k text, hash text, author text, name text, statements jsonb) RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    r uuid;
BEGIN
    PERFORM archive.ddl_allowed(k);
    INSERT INTO archive.revision (hash, author, name, statements) VALUES (hash, author, nullif(name, ''), statements) RETURNING id INTO r;
    RETURN r;
END $$;

GRANT USAGE ON SCHEMA archive TO "{p}_viewer";
GRANT SELECT ON archive.file, archive.member TO "{p}_viewer";
GRANT INSERT, UPDATE, DELETE ON archive.file TO "{p}_editor";
GRANT SELECT, INSERT, DELETE ON archive.secret TO "{p}_editor";
GRANT INSERT, UPDATE, DELETE ON archive.member TO "{p}_owner";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO "{p}_viewer";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT INSERT, UPDATE, DELETE ON TABLES TO "{p}_editor";

ALTER TABLE archive.file ENABLE ROW LEVEL SECURITY;
CREATE POLICY access ON archive.file USING (archive.can(access)) WITH CHECK (archive.can(access));

ALTER TABLE archive.secret ENABLE ROW LEVEL SECURITY;
CREATE POLICY owner ON archive.secret FOR SELECT USING (pg_has_role(current_user, '{p}_owner', 'USAGE'));
CREATE POLICY writer ON archive.secret FOR INSERT WITH CHECK (archive.can(access));
CREATE POLICY owner_delete ON archive.secret FOR DELETE USING (pg_has_role(current_user, '{p}_owner', 'USAGE'));
