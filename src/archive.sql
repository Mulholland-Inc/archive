CREATE SCHEMA archive;

-- What a workspace is for, which picks the icons the web app offers it.
CREATE TYPE archive.workspace_type AS ENUM ('personal', 'family', 'business');

-- icon and color are how the web app shows the workspace: one of its icons, and one of its 8 tints by index;
-- null is the default for its type.
CREATE TABLE archive.workspace (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL,
    key        bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    type       archive.workspace_type NOT NULL DEFAULT 'personal',
    icon       text CONSTRAINT workspace_icon CHECK (icon IN ('robot', 'chest', 'safe', 'bookcase', 'pinecone', 'cottage', 'castle',
        'townhouse', 'farmhouse', 'oak', 'pine', 'baobab', 'skyscraper', 'capitol', 'pyramid', 'ledger', 'cabinet', 'bank', 'warehouse', 'factory')),
    color      smallint CONSTRAINT workspace_color CHECK (color BETWEEN 0 AND 7)
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
    -- An API token's secret, sealed with the server's key, so its owner can copy it again; null for others.
    sealed     bytea,
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

-- A workflow is a row: a script Archive runs on its own, saved and changed with archive.create, update and
-- delete like any other row, through a write hook in Go (workflow.go) that compiles source and checks on
-- before anything is saved. Its owner reads and writes it: the member it belongs to and their tokens
-- (archive.mine). role is who it runs as: whoever saved it last, set by workflow_saved, so a token never
-- gains rights. trigger is on as Archive schedules it, hook its webhook secret, next_at when it is due, and
-- cursor_at and cursor_id the last row of its table it has seen; Archive keeps them, and they survive an edit
-- or a rename. member is null only on a workflow from before it had one, until Archive fills it in at open.
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
    member     uuid REFERENCES archive.member ON DELETE CASCADE,
    "on"       jsonb,
    UNIQUE (user_id, name)
);
CREATE INDEX workflow_next_idx ON archive.workflow (next_at) WHERE next_at IS NOT NULL;
COMMENT ON TABLE archive.workflow IS 'A script Archive runs on its own: on a schedule, when rows of a table are added or changed, when its webhook URL is called, or when run with archive.run. Each person sees and changes only their own
icon: workflow';
COMMENT ON COLUMN archive.workflow.name IS 'Letters, digits, - and _, like overdue-invoices; unique among yours';
COMMENT ON COLUMN archive.workflow.source IS 'Its TypeScript, written like a run';
COMMENT ON COLUMN archive.workflow."on" IS 'When it runs: { cron, tz? }, { table, events }, { webhook: true }, or null to run only with archive.run';
COMMENT ON COLUMN archive.workflow.next_at IS 'When it runs next: a cron workflow''s next time, or now for a table workflow with rows waiting';

-- A run of a workflow: what started it, what it returned, logged and threw, and how long it took. Archive
-- writes them, keeping the last 100 of each workflow; everyone else only reads them, as the workflow's owner.
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
COMMENT ON TABLE archive.workflow_run IS 'A run of one of your workflows, newest kept: the last 100 of each. Read only; Archive records them
icon: list-checks';
COMMENT ON COLUMN archive.workflow_run.trigger IS 'What started it: cron, table, webhook, or run (archive.run, or the Run button in the app; start in runs from before it was called run)';
COMMENT ON COLUMN archive.workflow_run.result IS 'What it returned; null when it failed';
COMMENT ON COLUMN archive.workflow_run.logs IS 'Its console lines';
COMMENT ON COLUMN archive.workflow_run.error IS 'Why it failed; empty when it worked, and then everything it wrote was saved';
COMMENT ON COLUMN archive.workflow_run.ms IS 'How long it took, in milliseconds';
COMMENT ON COLUMN archive.workflow_run.created_at IS 'When it started';

-- A connection is an account at another service that Archive reads: a member's own, like their Gmail, or, for
-- an integration, the archive's, like its QuickBooks company, which a connector's file says it is
-- (connect.Connector.Integration). An archive has at most one connection to each integration, which an owner
-- connects; member is then who connected it last, and every member sees it, and its records, while archive.can
-- their access, the group it is narrowed to, null for everyone, as rows of a user table are. Go alone writes it (connection.go): connecting adds it, connecting the same account again renews it,
-- and disconnecting deletes it. provider is a connector's id; token is its refresh token, sealed with the
-- workspace key like a secret value, and stays out of the catalog with member. meta is what the provider's
-- callback said that calls need, like QuickBooks' realmId. error is why its last refresh or sync failed,
-- empty while it works. Each member sees their own personal connections (archive.mine), owners too.
--
-- Its sync (sync.go) keeps a copy of what the account has in archive.record, a page at a time, and runs only
-- when something asks: connecting, Sync now, a push from the provider, or a member reading record or opening
-- the workspace while it is stale. asked_at is when something last asked, and each stream catches up to it;
-- sync_at is when its next page is due, null once every stream has caught up, and lease_at when the page under
-- way may be taken over, so a connection runs one page at a time. synced_at is when every stream last caught
-- up, revoked says its grant was refused, which nothing but connecting again or Sync now retries, and cursor
-- is where each stream is, { <stream>: { cursor, at, more, since, skip, fails, error, done } }, which stays out
-- of the catalog too.
CREATE TABLE archive.connection (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    member     uuid NOT NULL REFERENCES archive.member ON DELETE CASCADE,
    provider   text NOT NULL,
    account    text NOT NULL,
    token      bytea NOT NULL,
    scopes     text[] NOT NULL DEFAULT '{}',
    error      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    meta       jsonb NOT NULL DEFAULT '{}',
    sync_at    timestamptz DEFAULT now(),
    synced_at  timestamptz,
    cursor     jsonb NOT NULL DEFAULT '{}',
    asked_at   timestamptz NOT NULL DEFAULT now(),
    lease_at   timestamptz,
    revoked    boolean NOT NULL DEFAULT false,
    integration boolean NOT NULL DEFAULT false,
    access     text,
    UNIQUE (member, provider, account)
);
CREATE UNIQUE INDEX connection_integration_idx ON archive.connection (provider) WHERE integration;
CREATE INDEX connection_sync_idx ON archive.connection (sync_at) WHERE sync_at IS NOT NULL;
COMMENT ON TABLE archive.connection IS 'One of your own accounts at another service that Archive reads for you, like your Gmail; archive.action uses it. Read only; connect and disconnect them in Settings, Connectors
icon: plug';
COMMENT ON COLUMN archive.connection.provider IS 'Which service, a connector: gmail, notion, quickbooks, ...';
COMMENT ON COLUMN archive.connection.meta IS 'What the service said about the account when it connected, like QuickBooks'' realmId';
COMMENT ON COLUMN archive.connection.account IS 'The account there, like the mailbox address';
COMMENT ON COLUMN archive.connection.scopes IS 'What Archive may do there, as the service granted it';
COMMENT ON COLUMN archive.connection.error IS 'Why Archive last failed to reach it, like a revoked grant; empty while it works';
COMMENT ON COLUMN archive.connection.sync_at IS 'When Archive next reads what changed there into record; null once it has it all, until the service says something changed, you ask, or you read record';
COMMENT ON COLUMN archive.connection.synced_at IS 'When Archive last had everything there in record';
COMMENT ON COLUMN archive.connection.integration IS 'It is the archive''s, like its QuickBooks, which an owner connects for everyone, not one person''s own';
COMMENT ON COLUMN archive.connection.access IS 'The group an integration and its records are narrowed to, like finance; null for everyone';

-- A record is a copy of one thing a connection's account has, like a Gmail message, kept by its sync
-- (sync.go): one row for each source_id, its id at the provider, of each stream, a kind of thing a connector
-- syncs (connect/sync.go). access is its connection's, which an integration's owner narrows. data is it as the
-- stream reads it, updated_at when it last changed there, or when
-- the sync saw it change, and deleted says it is gone there. seen_at is when a sync last read it, which a
-- stream that can only list everything again compares, and stays out of the catalog. Only Go writes them;
-- each member reads their own connections' records, and disconnecting deletes them.
CREATE TABLE archive.record (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    connection uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    stream     text NOT NULL,
    source_id  text NOT NULL,
    data       jsonb NOT NULL DEFAULT '{}',
    deleted    boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    seen_at    timestamptz NOT NULL DEFAULT now(),
    access     text,
    UNIQUE (connection, stream, source_id)
);
CREATE INDEX record_updated_idx ON archive.record (connection, stream, updated_at);
COMMENT ON TABLE archive.record IS 'A copy of one thing one of your connections has, like a Gmail message, which Archive keeps in step with it. Read only; Archive syncs them, and disconnecting deletes them
icon: refresh-cw';
COMMENT ON COLUMN archive.record.connection IS 'references connection; the account it is from';
COMMENT ON COLUMN archive.record.stream IS 'What kind of thing it is, a stream of the connection''s connector, like messages of gmail or issues of github';
COMMENT ON COLUMN archive.record.source_id IS 'Its id there';
COMMENT ON COLUMN archive.record.data IS 'It, as the service has it, in the shape of its stream';
COMMENT ON COLUMN archive.record.deleted IS 'It is gone there; the last data seen stays';
COMMENT ON COLUMN archive.record.updated_at IS 'When it last changed there, or when Archive saw it change';
COMMENT ON COLUMN archive.record.access IS 'Group that sees it, its connection''s: null for everyone who sees the connection';

-- What records are in the domains they belong to: mail, docs and payroll, in tables every connector of a domain
-- shares, as its file maps its records (connect/tables.go), so Gmail and Outlook are one mail and two payroll
-- providers one payroll. Only Go writes them (tables.go), with the records they are from, and each row goes
-- with its record: a record deleted there, or its connection disconnected, deletes it. Each is seen by whoever
-- sees its record, as record's policy has it.
--
-- People and organizations are shared by every domain and every connection, by a key that is the same wherever
-- they are seen: known_person is an email address, known_org an EIN or a domain. A row of one holds that key
-- alone, and is seen by whoever sees a row that names it; the rest, like a name, stays on the rows of the
-- records that said it. So a member sees a person only as their own connections and the integrations they may
-- see know them, and learns nothing of who else knows them.
CREATE TABLE archive.known_person (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      archive.email NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE archive.known_person IS 'A person your connections know, by their email address: one row however many connections know them, like a Gmail sender who is also a Gusto employee. Read only; Archive syncs them, and their names are where they were seen, like mail_party and payroll_worker
icon: contact';
COMMENT ON COLUMN archive.known_person.email IS 'Their address, lowercase: what makes them one person everywhere';

CREATE TABLE archive.known_org (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    ein        archive.us_ein UNIQUE,
    domain     archive.domain UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT known_org_key CHECK (ein IS NOT NULL OR domain IS NOT NULL)
);
COMMENT ON TABLE archive.known_org IS 'An organization your connections know, by its EIN or its web domain: one row however many connections know it. Read only; Archive syncs them, and their names are where they were seen, like payroll_company
icon: building-2';
COMMENT ON COLUMN archive.known_org.ein IS 'Its US employer identification number';
COMMENT ON COLUMN archive.known_org.domain IS 'Its web domain, like example.com';

CREATE TABLE archive.mail_message (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    record      uuid NOT NULL UNIQUE REFERENCES archive.record ON DELETE CASCADE,
    connection  uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    thread      text NOT NULL DEFAULT '',
    subject     text NOT NULL DEFAULT '',
    snippet     text NOT NULL DEFAULT '',
    sent_at     timestamptz,
    labels      text[] NOT NULL DEFAULT '{}',
    attachments integer NOT NULL DEFAULT 0
);
CREATE INDEX mail_message_sent_idx ON archive.mail_message (connection, sent_at);
COMMENT ON TABLE archive.mail_message IS 'A message of a mailbox you connected, like your Gmail; who it is from and to is in mail_party, and its text in its record. Read only; Archive syncs them
icon: mail';
COMMENT ON COLUMN archive.mail_message.record IS 'references record; the copy of it Archive keeps, whose data has its text';
COMMENT ON COLUMN archive.mail_message.connection IS 'references connection; the mailbox it is in';
COMMENT ON COLUMN archive.mail_message.thread IS 'The conversation it is part of, its id there';
COMMENT ON COLUMN archive.mail_message.sent_at IS 'When it was sent, as its Date says, or else when it arrived';
COMMENT ON COLUMN archive.mail_message.labels IS 'Its labels or folders there, like INBOX';
COMMENT ON COLUMN archive.mail_message.attachments IS 'How many files it has attached';

CREATE TABLE archive.mail_party (
    id      uuid PRIMARY KEY DEFAULT uuidv7(),
    message uuid NOT NULL REFERENCES archive.mail_message ON DELETE CASCADE,
    person  uuid NOT NULL REFERENCES archive.known_person ON DELETE CASCADE,
    role    text NOT NULL CONSTRAINT mail_party_role CHECK (role IN ('from', 'to', 'cc', 'bcc', 'reply_to')),
    name    archive.name,
    UNIQUE (message, role, person)
);
CREATE INDEX mail_party_person_idx ON archive.mail_party (person);
COMMENT ON TABLE archive.mail_party IS 'Someone a message is from or to. Read only; Archive syncs them
icon: at-sign';
COMMENT ON COLUMN archive.mail_party.message IS 'references mail_message';
COMMENT ON COLUMN archive.mail_party.person IS 'references known_person; who, by their address';
COMMENT ON COLUMN archive.mail_party.role IS 'from, to, cc, bcc or reply_to';
COMMENT ON COLUMN archive.mail_party.name IS 'The name the message gives them';

CREATE TABLE archive.doc_page (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    record     uuid NOT NULL UNIQUE REFERENCES archive.record ON DELETE CASCADE,
    connection uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    kind       text NOT NULL DEFAULT 'page',
    title      text NOT NULL DEFAULT '',
    url        archive.url,
    icon       text NOT NULL DEFAULT '',
    edited_at  timestamptz
);
CREATE INDEX doc_page_connection_idx ON archive.doc_page (connection);
COMMENT ON TABLE archive.doc_page IS 'A page of docs or a wiki you connected, like Notion; its content stays there. Read only; Archive syncs them
icon: file-text';
COMMENT ON COLUMN archive.doc_page.record IS 'references record; the copy of it Archive keeps';
COMMENT ON COLUMN archive.doc_page.connection IS 'references connection; the account it is in';
COMMENT ON COLUMN archive.doc_page.kind IS 'What it is there, like page or database';
COMMENT ON COLUMN archive.doc_page.edited_at IS 'When it was last edited, when the service says';

CREATE TABLE archive.payroll_company (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    record      uuid NOT NULL UNIQUE REFERENCES archive.record ON DELETE CASCADE,
    connection  uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    source_id   text NOT NULL,
    org         uuid REFERENCES archive.known_org ON DELETE SET NULL,
    name        text NOT NULL DEFAULT '',
    trade_name  text NOT NULL DEFAULT '',
    entity_type text NOT NULL DEFAULT '',
    UNIQUE (connection, source_id)
);
CREATE INDEX payroll_company_org_idx ON archive.payroll_company (org);
COMMENT ON TABLE archive.payroll_company IS 'A company a payroll you connected pays people for, like one in Gusto. Read only; Archive syncs them
icon: landmark';
COMMENT ON COLUMN archive.payroll_company.record IS 'references record; the copy of it Archive keeps';
COMMENT ON COLUMN archive.payroll_company.connection IS 'references connection; the payroll it is in';
COMMENT ON COLUMN archive.payroll_company.source_id IS 'Its id there';
COMMENT ON COLUMN archive.payroll_company.org IS 'references known_org; the organization it is, by its EIN';
COMMENT ON COLUMN archive.payroll_company.trade_name IS 'The name it does business as';
COMMENT ON COLUMN archive.payroll_company.entity_type IS 'What kind of company it is, like LLC';

CREATE TABLE archive.payroll_worker (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    record         uuid NOT NULL UNIQUE REFERENCES archive.record ON DELETE CASCADE,
    connection     uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    source_id      text NOT NULL,
    kind           text NOT NULL CONSTRAINT payroll_worker_kind CHECK (kind IN ('employee', 'contractor')),
    company        uuid REFERENCES archive.payroll_company ON DELETE SET NULL,
    company_source text NOT NULL DEFAULT '',
    person         uuid REFERENCES archive.known_person ON DELETE SET NULL,
    name           archive.name,
    business_name  text NOT NULL DEFAULT '',
    email          archive.email,
    personal_email archive.email,
    title          text NOT NULL DEFAULT '',
    hired_on       date,
    terminated_on  date,
    active         boolean NOT NULL DEFAULT true,
    pay_rate       archive.money,
    pay_per        text NOT NULL DEFAULT '',
    UNIQUE (connection, source_id)
);
CREATE INDEX payroll_worker_person_idx ON archive.payroll_worker (person);
CREATE INDEX payroll_worker_company_idx ON archive.payroll_worker (company);
COMMENT ON TABLE archive.payroll_worker IS 'Someone a payroll you connected pays: an employee or a contractor, the ones who left too. Read only; Archive syncs them
icon: id-card';
COMMENT ON COLUMN archive.payroll_worker.record IS 'references record; the copy of them Archive keeps';
COMMENT ON COLUMN archive.payroll_worker.connection IS 'references connection; the payroll they are in';
COMMENT ON COLUMN archive.payroll_worker.source_id IS 'Their id there';
COMMENT ON COLUMN archive.payroll_worker.kind IS 'employee or contractor';
COMMENT ON COLUMN archive.payroll_worker.company IS 'references payroll_company; who they work for';
COMMENT ON COLUMN archive.payroll_worker.company_source IS 'The id there of who they work for';
COMMENT ON COLUMN archive.payroll_worker.person IS 'references known_person; who they are, by their work email, or else their own';
COMMENT ON COLUMN archive.payroll_worker.business_name IS 'A contractor''s business';
COMMENT ON COLUMN archive.payroll_worker.email IS 'The address they are known by: their work email, or else their own';
COMMENT ON COLUMN archive.payroll_worker.personal_email IS 'Their own address, when they have a work email too';
COMMENT ON COLUMN archive.payroll_worker.title IS 'Their job title';
COMMENT ON COLUMN archive.payroll_worker.hired_on IS 'When they started';
COMMENT ON COLUMN archive.payroll_worker.terminated_on IS 'When they left, when they did';
COMMENT ON COLUMN archive.payroll_worker.active IS 'They work there now';
COMMENT ON COLUMN archive.payroll_worker.pay_rate IS 'What they are paid, per pay_per';
COMMENT ON COLUMN archive.payroll_worker.pay_per IS 'What pay_rate is for, like Hour, Year or Paycheck';

CREATE TABLE archive.payroll_run (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    record         uuid NOT NULL UNIQUE REFERENCES archive.record ON DELETE CASCADE,
    connection     uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    source_id      text NOT NULL,
    company        uuid REFERENCES archive.payroll_company ON DELETE SET NULL,
    company_source text NOT NULL DEFAULT '',
    period_start   date,
    period_end     date,
    check_date     date,
    processed      boolean NOT NULL DEFAULT false,
    off_cycle      boolean NOT NULL DEFAULT false,
    external       boolean NOT NULL DEFAULT false,
    gross_pay      archive.money,
    net_pay        archive.money,
    employee_taxes archive.money,
    employer_taxes archive.money,
    UNIQUE (connection, source_id)
);
CREATE INDEX payroll_run_company_idx ON archive.payroll_run (company, check_date);
COMMENT ON TABLE archive.payroll_run IS 'A payroll: a company paying its people for a pay period, on its check date; what each was paid is in payroll_pay. Read only; Archive syncs them
icon: banknote';
COMMENT ON COLUMN archive.payroll_run.record IS 'references record; the copy of it Archive keeps';
COMMENT ON COLUMN archive.payroll_run.connection IS 'references connection; the payroll it is in';
COMMENT ON COLUMN archive.payroll_run.source_id IS 'Its id there';
COMMENT ON COLUMN archive.payroll_run.company IS 'references payroll_company';
COMMENT ON COLUMN archive.payroll_run.company_source IS 'The id there of its company';
COMMENT ON COLUMN archive.payroll_run.check_date IS 'When its people are paid';
COMMENT ON COLUMN archive.payroll_run.processed IS 'It was run; until then its amounts may change';
COMMENT ON COLUMN archive.payroll_run.off_cycle IS 'It is off its schedule, like a bonus or a correction';
COMMENT ON COLUMN archive.payroll_run.external IS 'It was paid outside the payroll provider and recorded there';
COMMENT ON COLUMN archive.payroll_run.employee_taxes IS 'The taxes withheld from its people';
COMMENT ON COLUMN archive.payroll_run.employer_taxes IS 'The taxes the company pays on top';

CREATE TABLE archive.payroll_pay (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    record         uuid NOT NULL REFERENCES archive.record ON DELETE CASCADE,
    connection     uuid NOT NULL REFERENCES archive.connection ON DELETE CASCADE,
    run            uuid REFERENCES archive.payroll_run ON DELETE CASCADE,
    worker         uuid REFERENCES archive.payroll_worker ON DELETE SET NULL,
    worker_source  text NOT NULL,
    paid_on        date,
    gross_pay      archive.money,
    net_pay        archive.money,
    deductions     archive.money,
    employee_taxes archive.money,
    employer_taxes archive.money,
    payment_method text NOT NULL DEFAULT '',
    UNIQUE (record, worker_source)
);
CREATE INDEX payroll_pay_run_idx ON archive.payroll_pay (run);
CREATE INDEX payroll_pay_worker_idx ON archive.payroll_pay (connection, worker_source);
COMMENT ON TABLE archive.payroll_pay IS 'What one person was paid: in a payroll, or on its own, like a contractor payment. Read only; Archive syncs them
icon: receipt';
COMMENT ON COLUMN archive.payroll_pay.record IS 'references record; the copy Archive keeps of the payroll or payment it is in';
COMMENT ON COLUMN archive.payroll_pay.connection IS 'references connection; the payroll it is in';
COMMENT ON COLUMN archive.payroll_pay.run IS 'references payroll_run; the payroll it is part of, none for a payment on its own';
COMMENT ON COLUMN archive.payroll_pay.worker IS 'references payroll_worker; who was paid';
COMMENT ON COLUMN archive.payroll_pay.worker_source IS 'The id there of who was paid';
COMMENT ON COLUMN archive.payroll_pay.paid_on IS 'When it was paid';
COMMENT ON COLUMN archive.payroll_pay.gross_pay IS 'What they earned, before taxes and deductions';
COMMENT ON COLUMN archive.payroll_pay.net_pay IS 'What they took home';
COMMENT ON COLUMN archive.payroll_pay.deductions IS 'What was taken off for benefits, garnishments and the like';
COMMENT ON COLUMN archive.payroll_pay.employee_taxes IS 'The taxes withheld from them';
COMMENT ON COLUMN archive.payroll_pay.employer_taxes IS 'The taxes the company paid on top';
COMMENT ON COLUMN archive.payroll_pay.payment_method IS 'How they were paid, like Direct Deposit or Check';

-- money_of is an amount in minor units in currency as archive.money, null when there is no amount.
CREATE FUNCTION archive.money_of(minor bigint, currency text) RETURNS archive.money LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE WHEN minor IS NOT NULL THEN ROW(minor, currency)::archive.money END
$$;

-- An OAuth client Archive registered for itself at an MCP connector's authorization server (connect/mcp.go):
-- one per connector, kept in the main database for every workspace, since the redirect URI is the server's.
-- client is the connect.Registration, its secret with it, sealed with ARCHIVE_SECRET_KEY. Only Go reads it.
CREATE TABLE archive.oauth_client (
    provider   text PRIMARY KEY,
    client     bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- A watch is something a connector's provider pushes for a connection's account (connect/push.go): a Gmail
-- users.watch, a Drive channel, a Graph subscription, an Asana webhook, or the key a push of the app's own
-- webhook names it by, like a QuickBooks realmId. key is what a push names it by, resource what the provider
-- calls it, secret what its pushes prove themselves with, sealed with ARCHIVE_SECRET_KEY, and expires_at when
-- it lapses unless a sync renews it. Kept in the main database for every workspace, since a push says only
-- the key; only Go reads it.
CREATE TABLE archive.watch (
    provider   text NOT NULL,
    key        text NOT NULL,
    workspace  uuid NOT NULL,
    connection uuid NOT NULL,
    resource   text NOT NULL DEFAULT '',
    secret     bytea,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, key, connection)
);
CREATE INDEX watch_connection_idx ON archive.watch (connection);

-- mine says whether the current role is the member m or one of their tokens, which are all in m's group role.
CREATE FUNCTION archive.mine(m uuid) RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT EXISTS (SELECT FROM pg_catalog.pg_roles r WHERE r.rolname = '{p}_m' || translate(rtrim(encode(uuid_send(m), 'base64'), '='), '+/', '-_')
        AND pg_has_role(current_user, r.oid, 'USAGE'))
$$;

-- workflow_saved makes a workflow run as whoever saved it last. Archive's own writes (scheduling, the cursor)
-- run with no role set and leave it.
CREATE FUNCTION archive.workflow_saved() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF current_setting('role') <> 'none' THEN
        NEW.role := current_user;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER workflow_saved BEFORE INSERT OR UPDATE ON archive.workflow FOR EACH ROW EXECUTE FUNCTION archive.workflow_saved();

-- moved says what a write moved to the session that wrote it, on a channel only that session listens on. A
-- notification is sent only when its transaction commits and is dropped with a rolled-back savepoint, so Go
-- hears exactly what committed (live.go). A public table names itself; an archive table its schema too and
-- the row's id. Every public table gets the statement trigger archive_moved (schemaDerive).
CREATE FUNCTION archive.moved() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    what text := CASE WHEN TG_TABLE_SCHEMA = 'public' THEN TG_TABLE_NAME ELSE TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME END;
BEGIN
    IF TG_LEVEL = 'ROW' AND TG_OP = 'DELETE' THEN
        what := what || ' ' || OLD.id;
    ELSIF TG_LEVEL = 'ROW' THEN
        what := what || ' ' || NEW.id;
    END IF;
    PERFORM pg_notify('archive_moved_' || pg_backend_pid(), what);
    RETURN NULL;
END $$;

CREATE TRIGGER moved AFTER INSERT OR UPDATE OR DELETE ON archive.chat FOR EACH ROW EXECUTE FUNCTION archive.moved();
CREATE TRIGGER moved AFTER INSERT OR UPDATE OR DELETE ON archive.workflow FOR EACH ROW EXECUTE FUNCTION archive.moved();
CREATE TRIGGER moved AFTER INSERT OR UPDATE OR DELETE ON archive.workflow_run FOR EACH ROW EXECUTE FUNCTION archive.moved();
CREATE TRIGGER moved AFTER INSERT OR UPDATE OR DELETE ON archive.connection FOR EACH ROW EXECUTE FUNCTION archive.moved();
CREATE TRIGGER moved AFTER INSERT OR UPDATE OR DELETE ON archive.file FOR EACH ROW EXECUTE FUNCTION archive.moved();
CREATE TRIGGER moved AFTER INSERT OR UPDATE OR DELETE ON archive.member FOR EACH ROW EXECUTE FUNCTION archive.moved();
CREATE TRIGGER moved AFTER INSERT OR UPDATE OR DELETE ON archive.revision FOR EACH ROW EXECUTE FUNCTION archive.moved();

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
GRANT SELECT, INSERT, UPDATE, DELETE ON archive.workflow TO "{p}_viewer";
GRANT SELECT ON archive.workflow_run TO "{p}_viewer";
GRANT SELECT ON archive.connection TO "{p}_viewer";
GRANT SELECT ON archive.record TO "{p}_viewer";
GRANT SELECT ON archive.known_person, archive.known_org, archive.mail_message, archive.mail_party, archive.doc_page, archive.payroll_company,
    archive.payroll_worker, archive.payroll_run, archive.payroll_pay TO "{p}_viewer";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO "{p}_viewer";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT INSERT, UPDATE, DELETE ON TABLES TO "{p}_editor";

ALTER TABLE archive.file ENABLE ROW LEVEL SECURITY;
CREATE POLICY access ON archive.file USING (archive.can(access)) WITH CHECK (archive.can(access));

ALTER TABLE archive.secret ENABLE ROW LEVEL SECURITY;
CREATE POLICY owner ON archive.secret FOR SELECT USING (pg_has_role(current_user, '{p}_owner', 'USAGE'));
CREATE POLICY writer ON archive.secret FOR INSERT WITH CHECK (archive.can(access));
CREATE POLICY owner_delete ON archive.secret FOR DELETE USING (pg_has_role(current_user, '{p}_owner', 'USAGE'));

ALTER TABLE archive.workflow ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.workflow USING (archive.mine(member)) WITH CHECK (archive.mine(member));

ALTER TABLE archive.workflow_run ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.workflow_run FOR SELECT USING (EXISTS (SELECT FROM archive.workflow w WHERE w.id = workflow_run.workflow));

ALTER TABLE archive.connection ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.connection FOR SELECT USING (archive.mine(member) AND NOT integration OR integration AND archive.can(access));

ALTER TABLE archive.record ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.record FOR SELECT USING (archive.can(access) AND EXISTS (SELECT FROM archive.connection c WHERE c.id = record.connection));

-- A row of a record's domain is seen with its record. A person or an organization is seen with a row that names
-- it, and with nothing else, so it says no more than what a member could already see.
ALTER TABLE archive.mail_message ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.mail_message FOR SELECT USING (EXISTS (SELECT FROM archive.record r WHERE r.id = mail_message.record));
ALTER TABLE archive.doc_page ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.doc_page FOR SELECT USING (EXISTS (SELECT FROM archive.record r WHERE r.id = doc_page.record));
ALTER TABLE archive.payroll_company ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.payroll_company FOR SELECT USING (EXISTS (SELECT FROM archive.record r WHERE r.id = payroll_company.record));
ALTER TABLE archive.payroll_worker ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.payroll_worker FOR SELECT USING (EXISTS (SELECT FROM archive.record r WHERE r.id = payroll_worker.record));
ALTER TABLE archive.payroll_run ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.payroll_run FOR SELECT USING (EXISTS (SELECT FROM archive.record r WHERE r.id = payroll_run.record));
ALTER TABLE archive.payroll_pay ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.payroll_pay FOR SELECT USING (EXISTS (SELECT FROM archive.record r WHERE r.id = payroll_pay.record));
ALTER TABLE archive.mail_party ENABLE ROW LEVEL SECURITY;
CREATE POLICY mine ON archive.mail_party FOR SELECT USING (EXISTS (SELECT FROM archive.mail_message m WHERE m.id = mail_party.message));
ALTER TABLE archive.known_person ENABLE ROW LEVEL SECURITY;
CREATE POLICY seen ON archive.known_person FOR SELECT USING (EXISTS (SELECT FROM archive.mail_party p WHERE p.person = known_person.id)
    OR EXISTS (SELECT FROM archive.payroll_worker w WHERE w.person = known_person.id));
ALTER TABLE archive.known_org ENABLE ROW LEVEL SECURITY;
CREATE POLICY seen ON archive.known_org FOR SELECT USING (EXISTS (SELECT FROM archive.payroll_company c WHERE c.org = known_org.id));

-- name: HasRole :one
SELECT pg_has_role(sqlc.arg(member)::name, rolname, 'USAGE') FROM pg_catalog.pg_roles WHERE rolname = sqlc.arg(grp)::name;

-- name: RoleExists :one
SELECT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = sqlc.arg(role)::name);

-- name: UserByEmail :one
SELECT * FROM archive."user" WHERE email = $1;

-- name: UserByID :one
SELECT * FROM archive."user" WHERE id = $1;

-- name: UserByOIDC :one
SELECT * FROM archive."user" WHERE signin->'oidc' @> sqlc.arg(link)::jsonb;

-- name: FirstUser :one
SELECT * FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid] ORDER BY created_at, id LIMIT 1;

-- name: NoMembers :one
SELECT NOT EXISTS (SELECT FROM archive.member);

-- name: MemberByEmail :one
SELECT id, role::text AS role, groups FROM archive.member WHERE email = sqlc.arg(email)::text;

-- name: Members :many
SELECT id, email::text AS email, role::text AS role, groups FROM archive.member ORDER BY email;

-- name: WorkspaceUsers :many
SELECT id, email, name FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid] ORDER BY email;

-- name: UserWorkspaces :many
SELECT w.* FROM archive.workspace w JOIN archive."user" u ON w.id = ANY(u.workspaces) WHERE u.id = $1 ORDER BY array_position(u.workspaces, w.id);

-- name: JoinWorkspace :one
INSERT INTO archive."user" (email, name, workspaces) VALUES (sqlc.arg(email), sqlc.arg(name), ARRAY[sqlc.arg(workspace)::uuid])
ON CONFLICT (email) DO UPDATE SET workspaces = archive."user".workspaces || EXCLUDED.workspaces WHERE NOT archive."user".workspaces @> EXCLUDED.workspaces
RETURNING *;

-- name: LeaveWorkspace :exec
UPDATE archive."user" SET workspaces = array_remove(workspaces, sqlc.arg(workspace)::uuid) WHERE id = sqlc.arg(id);

-- name: SetSignin :exec
UPDATE archive."user" SET signin = $2 WHERE id = $1;

-- name: NameUser :exec
UPDATE archive."user" SET name = $2 WHERE id = $1 AND name = '';

-- name: Authenticate :one
SELECT t.id, t.kind, t.name, coalesce(t.role, '')::text AS role, u.id AS user_id, u.email, u.name AS user_name,
    coalesce(t.workspace::text, '')::text AS workspace,
    ARRAY(SELECT w.id::text FROM archive.workspace w WHERE w.id = ANY(u.workspaces) ORDER BY array_position(u.workspaces, w.id))::text[] AS workspaces
FROM archive.token t JOIN archive."user" u ON u.id = t.user_id
WHERE t.hash = $1 AND t.kind IN ('api', 'session') AND (t.expires_at IS NULL OR t.expires_at > now());

-- name: CreateSession :exec
INSERT INTO archive.token (user_id, kind, hash, expires_at) VALUES ($1, 'session', $2, now() + interval '30 days');

-- name: SignOut :exec
DELETE FROM archive.token WHERE hash = $1 AND kind = 'session';

-- name: CreateToken :one
INSERT INTO archive.token (id, user_id, workspace, kind, name, role, hash, sealed, expires_at) VALUES ($1, $2, $3, 'api', $4, $5, $6, $7, $8) RETURNING created_at, expires_at;

-- name: TokenSealed :one
SELECT sealed FROM archive.token WHERE id = $1 AND user_id = $2 AND workspace = $3 AND kind = 'api' AND sealed IS NOT NULL;

-- name: Tokens :many
SELECT id, name, (role IS NOT NULL)::bool AS viewer, expires_at, created_at FROM archive.token
WHERE user_id = $1 AND workspace = $2 AND kind = 'api' ORDER BY id;

-- name: RevokeToken :one
DELETE FROM archive.token WHERE id = $1 AND user_id = $2 AND workspace = $3 AND kind = 'api' RETURNING coalesce(role, '')::text;

-- name: DeleteWorkspaceTokens :many
DELETE FROM archive.token WHERE user_id = $1 AND workspace = $2 RETURNING coalesce(role, '')::text;

-- name: ForgetUser :exec
WITH gone AS (DELETE FROM archive.chat c WHERE c.user_id = sqlc.arg(id)::uuid) DELETE FROM archive.workflow w WHERE w.user_id = sqlc.arg(id)::uuid;

-- name: Chats :many
SELECT id, title, created_at, updated_at FROM archive.chat WHERE user_id = $1 ORDER BY updated_at DESC LIMIT 200;

-- name: ChatByID :one
SELECT * FROM archive.chat WHERE id = $1 AND user_id = $2;

-- name: CreateChat :one
INSERT INTO archive.chat (user_id, title) VALUES ($1, $2) RETURNING *;

-- name: AppendChat :exec
UPDATE archive.chat SET messages = messages || sqlc.arg(messages)::jsonb, updated_at = now() WHERE id = sqlc.arg(id);

-- name: RenameChat :execrows
UPDATE archive.chat SET title = $3 WHERE id = $1 AND user_id = $2;

-- name: DeleteChat :execrows
DELETE FROM archive.chat WHERE id = $1 AND user_id = $2;

-- name: InputErrors :many
SELECT i::bigint AS i, e.message::text AS message
FROM generate_subscripts(sqlc.arg(texts)::text[], 1) i, pg_input_error_info((sqlc.arg(texts)::text[])[i], (sqlc.arg(regs)::text[])[i]) e
WHERE e.message IS NOT NULL ORDER BY i;

-- name: ClearSecrets :exec
SELECT archive.secret_clear(sqlc.arg(tbl), sqlc.arg(rid), sqlc.narg(col));

-- name: CreateSecret :exec
INSERT INTO archive.secret (id, table_name, row_id, column_name, type, value, access) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: Secrets :many
SELECT id, row_id, column_name, type, value FROM archive.secret WHERE table_name = $1 AND row_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: FileByKey :one
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at FROM archive.file WHERE access IS NOT DISTINCT FROM $1 AND hash = $2 AND name = $3;

-- name: CreateFile :one
INSERT INTO archive.file (hash, name, mime, size, access) VALUES ($1, $2, $3, $4, $5)
RETURNING id, hash, name, mime, size, metadata, status, error, access, created_at;

-- name: Files :many
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at FROM archive.file
WHERE id > sqlc.arg(after) ORDER BY id LIMIT sqlc.arg(lim);

-- name: GetFile :one
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at FROM archive.file WHERE id = $1;

-- name: RenameFile :one
UPDATE archive.file SET name = $2 WHERE id = $1
RETURNING id, hash, name, mime, size, metadata, status, error, access, created_at;

-- name: LockFile :one
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at FROM archive.file WHERE id = $1 FOR UPDATE;

-- name: FileText :one
SELECT text FROM archive.file WHERE id = $1;

-- name: FileWithText :one
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at, text FROM archive.file WHERE id = $1;

-- name: FilesWithText :many
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at, text FROM archive.file
WHERE id > sqlc.arg(after) ORDER BY id LIMIT sqlc.arg(lim);

-- name: NextPendingFile :one
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at FROM archive.file
WHERE status = 'pending' ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED;

-- name: LockPendingFile :one
SELECT id, hash, name, mime, size, metadata, status, error, access, created_at FROM archive.file
WHERE id = $1 AND status = 'pending' FOR UPDATE;

-- name: SetFileContent :exec
UPDATE archive.file SET metadata = $2, text = $3 WHERE id = $1;

-- name: SetFileStatus :exec
UPDATE archive.file SET status = $2, error = $3 WHERE id = $1;

-- name: ResetProcessing :exec
UPDATE archive.file SET status = 'pending' WHERE status = 'processing';

-- name: ReplaceFile :one
UPDATE archive.file SET hash = $2, size = $3, status = 'pending', text = '', error = '', metadata = '{}' WHERE id = $1
RETURNING id, hash, name, mime, size, metadata, status, error, access, created_at;

-- name: Upload :one
SELECT * FROM archive.upload WHERE id = $1;

-- name: LockUpload :one
SELECT * FROM archive.upload WHERE id = $1 FOR UPDATE;

-- name: CreateUpload :exec
INSERT INTO archive.upload (id, role, access, name, length) VALUES ($1, $2, $3, $4, $5);

-- name: SetUploadParts :exec
UPDATE archive.upload SET parts = $2 WHERE id = $1;

-- name: DeleteUpload :exec
DELETE FROM archive.upload WHERE id = $1;

-- name: Revision :one
SELECT coalesce((SELECT id::text || ':' || hash FROM archive.revision ORDER BY created_at DESC, id DESC LIMIT 1), '')::text;

-- name: InsertRevision :exec
INSERT INTO archive.revision (hash) VALUES ($1);

-- name: Revisions :many
SELECT id, created_at, author, coalesce(name, '')::text AS name, statements FROM archive.revision WHERE statements <> '[]' ORDER BY created_at DESC, id DESC LIMIT $1;

-- name: RevisionNamed :one
SELECT EXISTS (SELECT FROM archive.revision WHERE name = $1);

-- name: AddDDLKey :exec
WITH old AS (DELETE FROM archive.ddl_key WHERE created_at < now() - interval '1 hour')
INSERT INTO archive.ddl_key (key, pid) VALUES ($1, $2);

-- name: DeleteDDLKey :exec
DELETE FROM archive.ddl_key WHERE key = $1;

-- name: DDL :exec
SELECT archive.ddl(sqlc.arg(key), sqlc.arg(stmt));

-- name: DDLRevision :one
SELECT archive.ddl_revision(sqlc.arg(key), sqlc.arg(hash), sqlc.arg(author), sqlc.arg(name), sqlc.arg(statements))::uuid AS id;

-- name: BackendPID :one
SELECT pg_backend_pid();

-- name: SetLocal :exec
SELECT set_config(sqlc.arg(name), sqlc.arg(value), true);

-- name: AdvisoryLock :exec
SELECT pg_advisory_lock(sqlc.arg(key));

-- name: AdvisoryUnlock :exec
SELECT pg_advisory_unlock(sqlc.arg(key));

-- name: AdvisoryXactLock :exec
SELECT pg_advisory_xact_lock(sqlc.arg(key));

-- name: Catalog :one
WITH col AS (
    SELECT a.attrelid AS rel, a.attnum AS num, jsonb_build_object(
        'name', a.attname,
        'description', coalesce(col_description(a.attrelid, a.attnum), ''),
        'type', e.typname,
        'schema', e.typnamespace::regnamespace::text,
        'reg', format_type(e.oid, a.atttypmod),
        'list', t.typcategory = 'A' AND t.typelem <> 0,
        'not_null', a.attnotnull,
        'default', a.atthasdef,
        'generated', a.attgenerated <> '',
        'pk', EXISTS (SELECT FROM pg_catalog.pg_constraint k WHERE k.conrelid = a.attrelid AND k.contype = 'p' AND k.conkey = ARRAY[a.attnum]),
        'ref', coalesce((SELECT r.relnamespace::regnamespace::text || '.' || r.relname FROM pg_catalog.pg_constraint k JOIN pg_catalog.pg_class r ON r.oid = k.confrelid
            WHERE k.conrelid = a.attrelid AND k.contype = 'f' AND k.conkey = ARRAY[a.attnum] LIMIT 1), '')) AS c
    FROM pg_catalog.pg_attribute a
    JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
    JOIN pg_catalog.pg_type e ON e.oid = CASE WHEN t.typcategory = 'A' AND t.typelem <> 0 THEN t.typelem ELSE t.oid END
    WHERE a.attnum > 0 AND NOT a.attisdropped
)
SELECT jsonb_build_object(
    'tables', coalesce((SELECT jsonb_object_agg(c.relname, jsonb_build_object(
        'name', c.relname,
        'schema', c.relnamespace::regnamespace::text,
        'kind', CASE c.relkind WHEN 'v' THEN 'view' ELSE 'table' END,
        'description', coalesce(obj_description(c.oid, 'pg_class'), ''),
        'rls', c.relrowsecurity,
        'force', c.relforcerowsecurity,
        'security_invoker', coalesce((SELECT o.option_value::bool FROM pg_options_to_table(c.reloptions) o WHERE o.option_name = 'security_invoker'), false),
        'policy', coalesce((SELECT count(*) = 1 AND bool_and(p.polname = 'access' AND p.polcmd = '*' AND p.polpermissive AND p.polroles = '{0}'::oid[]
            AND regexp_replace(pg_get_expr(p.polqual, p.polrelid), '^archive\.', '') = 'can(access)'
            AND regexp_replace(pg_get_expr(p.polwithcheck, p.polrelid), '^archive\.', '') = 'can(access)')
            FROM pg_catalog.pg_policy p WHERE p.polrelid = c.oid), false),
        'columns', (SELECT jsonb_agg(col.c ORDER BY col.num) FROM col WHERE col.rel = c.oid)))
        FROM pg_catalog.pg_class c WHERE (c.relnamespace = 'public'::regnamespace OR c.oid IN ('archive.member'::regclass, 'archive.workflow'::regclass, 'archive.workflow_run'::regclass, 'archive.connection'::regclass, 'archive.record'::regclass,
            'archive.known_person'::regclass, 'archive.known_org'::regclass, 'archive.mail_message'::regclass, 'archive.mail_party'::regclass, 'archive.doc_page'::regclass,
            'archive.payroll_company'::regclass, 'archive.payroll_worker'::regclass, 'archive.payroll_run'::regclass, 'archive.payroll_pay'::regclass)
            AND NOT EXISTS (SELECT FROM pg_catalog.pg_class p WHERE p.relnamespace = 'public'::regnamespace AND p.relname = c.relname)) AND c.relkind IN ('r', 'p', 'v')), '{}'),
    'types', coalesce((SELECT jsonb_object_agg(t.typnamespace::regnamespace::text || '.' || t.typname, jsonb_build_object(
        'name', t.typname,
        'schema', t.typnamespace::regnamespace::text,
        'kind', t.typtype,
        'description', coalesce(obj_description(t.oid, 'pg_type'), ''),
        'base', coalesce((SELECT b.typname FROM pg_catalog.pg_type b WHERE b.oid = t.typbasetype), ''),
        'base_schema', coalesce((SELECT b.typnamespace::regnamespace::text FROM pg_catalog.pg_type b WHERE b.oid = t.typbasetype), ''),
        'enum', coalesce((SELECT jsonb_agg(l.enumlabel ORDER BY l.enumsortorder) FROM pg_catalog.pg_enum l WHERE l.enumtypid = t.oid), '[]'),
        'attrs', coalesce((SELECT jsonb_agg(col.c ORDER BY col.num) FROM col WHERE col.rel = t.typrelid), '[]')))
        FROM pg_catalog.pg_type t LEFT JOIN pg_catalog.pg_class r ON r.oid = t.typrelid
        WHERE t.typnamespace IN ('archive'::regnamespace, 'public'::regnamespace)
          AND (t.typtype IN ('d', 'e') OR t.typtype = 'c' AND r.relkind = 'c')), '{}'));

-- name: WorkflowNames :many
SELECT name FROM archive.workflow WHERE user_id = $1 ORDER BY name;

-- name: WorkflowByName :one
SELECT * FROM archive.workflow WHERE user_id = $1 AND name = $2;

-- name: LockWorkflow :one
SELECT * FROM archive.workflow WHERE id = $1 FOR UPDATE;

-- name: WorkflowByHook :one
SELECT * FROM archive.workflow WHERE hook = $1 AND trigger = 'webhook';

-- name: InsertWorkflow :one
INSERT INTO archive.workflow (user_id, member, role, name, source, "on", trigger, hook, next_at, cursor_at, cursor_id)
VALUES (sqlc.arg(user_id), sqlc.arg(member), sqlc.arg(role), sqlc.arg(name), sqlc.arg(source), nullif(sqlc.arg(on_json)::jsonb, 'null'), sqlc.arg(trigger), sqlc.arg(hook),
    sqlc.narg(next_at), sqlc.narg(cursor_at), sqlc.narg(cursor_id))
RETURNING id;

-- name: UpdateWorkflow :execrows
UPDATE archive.workflow SET name = sqlc.arg(name), source = sqlc.arg(source), "on" = nullif(sqlc.arg(on_json)::jsonb, 'null'), trigger = sqlc.arg(trigger),
    next_at = sqlc.narg(next_at), cursor_at = sqlc.narg(cursor_at), cursor_id = sqlc.narg(cursor_id), updated_at = now()
WHERE id = sqlc.arg(id);

-- name: DeleteWorkflow :execrows
DELETE FROM archive.workflow WHERE id = $1;

-- Workflows from before they were rows: no member yet, or a trigger still in a // on: line of their source.
-- name: OldWorkflows :many
SELECT * FROM archive.workflow WHERE member IS NULL OR ("on" IS NULL AND trigger <> '');

-- name: UpgradeWorkflow :exec
UPDATE archive.workflow SET member = sqlc.narg(member), source = sqlc.arg(source), "on" = nullif(sqlc.arg(on_json)::jsonb, 'null'), trigger = sqlc.arg(trigger)
WHERE id = sqlc.arg(id);

-- name: ClaimWorkflow :one
UPDATE archive.workflow SET next_at = sqlc.arg(lease)::timestamptz
WHERE id = (SELECT w.id FROM archive.workflow w WHERE w.next_at <= now() ORDER BY w.next_at LIMIT 1 FOR UPDATE SKIP LOCKED)
RETURNING *;

-- name: SetWorkflowNext :exec
UPDATE archive.workflow SET next_at = $3 WHERE id = $1 AND trigger = $2;

-- name: AdvanceWorkflow :exec
UPDATE archive.workflow SET next_at = sqlc.arg(next_at), cursor_at = sqlc.arg(cursor_at), cursor_id = sqlc.arg(cursor_id)
WHERE id = sqlc.arg(id) AND trigger = sqlc.arg(trigger);

-- name: AddWorkflowRun :one
INSERT INTO archive.workflow_run (workflow, trigger, result, logs, error, ms, created_at)
SELECT w.id, sqlc.arg(trigger), sqlc.arg(result), sqlc.arg(logs), sqlc.arg(error), sqlc.arg(ms), sqlc.arg(created_at)
FROM archive.workflow w WHERE w.id = sqlc.arg(workflow)
RETURNING *;

-- name: TrimWorkflowRuns :exec
DELETE FROM archive.workflow_run d WHERE d.workflow = $1
    AND d.id NOT IN (SELECT r.id FROM archive.workflow_run r WHERE r.workflow = $1 ORDER BY r.created_at DESC, r.id DESC LIMIT 100);

-- name: WorkflowRunTimes :many
SELECT created_at FROM archive.workflow_run WHERE workflow = $1 AND created_at >= sqlc.arg(since)::timestamptz;

-- name: TxNow :one
SELECT now()::timestamptz;

-- name: TxWrote :one
SELECT (pg_current_xact_id_if_assigned() IS NOT NULL)::bool AS wrote, (coalesce(current_setting('archive.members', true), '') = '1')::bool AS members;

-- name: WakeTableWorkflows :execrows
UPDATE archive.workflow SET next_at = now() WHERE cursor_id IS NOT NULL AND next_at IS NULL;

-- name: NextWorkflowAt :one
SELECT next_at FROM archive.workflow WHERE next_at IS NOT NULL ORDER BY next_at LIMIT 1;

-- name: HasPendingFiles :one
SELECT EXISTS (SELECT FROM archive.file WHERE status = 'pending');

-- name: FirstWorkspace :one
SELECT * FROM archive.workspace ORDER BY created_at, id LIMIT 1;

-- The first workspace before its store migrated the main database, so only the columns it always had.
-- name: FirstWorkspaceKey :one
SELECT id, name, key, created_at FROM archive.workspace ORDER BY created_at, id LIMIT 1;

-- name: WorkspaceByID :one
SELECT * FROM archive.workspace WHERE id = $1;

-- name: AllWorkspaces :many
SELECT * FROM archive.workspace ORDER BY created_at, id;

-- name: CreateWorkspace :one
INSERT INTO archive.workspace (id, name, key, type, icon, color) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING RETURNING *;

-- name: DeleteWorkspace :exec
WITH gone AS (UPDATE archive."user" u SET workspaces = array_remove(u.workspaces, sqlc.arg(id)::uuid) WHERE u.workspaces @> ARRAY[sqlc.arg(id)::uuid])
DELETE FROM archive.workspace w WHERE w.id = sqlc.arg(id)::uuid;

-- name: RolesWithPrefix :many
SELECT rolname::text FROM pg_catalog.pg_roles WHERE starts_with(rolname, sqlc.arg(prefix)::text);

-- name: UpdateWorkspace :exec
UPDATE archive.workspace SET name = coalesce(sqlc.narg(name), name), type = coalesce(sqlc.narg(type), type),
    icon = coalesce(sqlc.narg(icon), icon), color = coalesce(sqlc.narg(color), color) WHERE id = sqlc.arg(id);

-- name: SoonDue :one
WITH was AS (SELECT x.at FROM archive.due x WHERE x.workspace = sqlc.arg(workspace) AND x.kind = sqlc.arg(kind))
INSERT INTO archive.due (workspace, kind, at) VALUES (sqlc.arg(workspace), sqlc.arg(kind), sqlc.arg(at))
ON CONFLICT (workspace, kind) DO UPDATE SET at = least(archive.due.at, EXCLUDED.at)
RETURNING at, (at IS DISTINCT FROM (SELECT was.at FROM was))::bool AS changed;

-- name: ClaimDueFor :one
UPDATE archive.due d SET at = sqlc.arg(lease)::timestamptz
WHERE d.workspace = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at <= now()
RETURNING d.at;

-- name: DueAt :one
SELECT at FROM archive.due WHERE workspace = $1 AND kind = $2;

-- name: AllDue :many
SELECT workspace, kind, at FROM archive.due;

-- name: SetDue :execrows
UPDATE archive.due d SET at = sqlc.arg(next)::timestamptz FROM archive.workspace w
WHERE w.id = d.workspace AND w.id = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at = sqlc.arg(claimed)::timestamptz;

-- name: ClearDue :exec
DELETE FROM archive.due d USING archive.workspace w
WHERE w.id = d.workspace AND w.id = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at = sqlc.arg(claimed)::timestamptz;

-- name: Connections :many
SELECT c.id, c.provider, c.account, c.scopes, c.error, c.created_at, c.updated_at, c.sync_at, c.synced_at,
    (SELECT count(*) FROM archive.record r WHERE r.connection = c.id AND NOT r.deleted)::bigint AS records,
    c.integration, coalesce(c.access, '')::text AS access, coalesce((SELECT m.email::text FROM archive.member m WHERE m.id = c.member), '')::text AS connected_by
FROM archive.connection c WHERE c.member = $1 AND NOT c.integration OR c.integration ORDER BY c.provider, c.account, c.id;

-- name: Connection :one
SELECT * FROM archive.connection WHERE id = $1;

-- name: ConnectionFor :one
SELECT id FROM archive.connection WHERE member = $1 AND provider = $2 AND account = $3 AND NOT integration FOR UPDATE;

-- name: IntegrationFor :one
SELECT id, account FROM archive.connection WHERE provider = $1 AND integration FOR UPDATE;

-- name: SaveConnection :exec
INSERT INTO archive.connection (id, member, provider, account, token, scopes, meta, integration) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET member = EXCLUDED.member, token = EXCLUDED.token, scopes = EXCLUDED.scopes, meta = EXCLUDED.meta, error = '', updated_at = now(),
    sync_at = now(), asked_at = now(), revoked = false;

-- name: ConnectionLocked :one
SELECT * FROM archive.connection WHERE id = $1 FOR UPDATE;

-- A member's personal connections, and the archive's integrations.
-- name: MemberConnections :many
SELECT * FROM archive.connection WHERE member = $1 AND NOT integration OR integration ORDER BY provider, account, id;

-- name: OAuthClient :one
SELECT client FROM archive.oauth_client WHERE provider = $1;

-- name: SetOAuthClient :exec
INSERT INTO archive.oauth_client (provider, client) VALUES (sqlc.arg(provider), sqlc.arg(client))
ON CONFLICT (provider) DO UPDATE SET client = EXCLUDED.client, created_at = now() WHERE archive.oauth_client.client = sqlc.narg(old);

-- name: DeleteOAuthClient :exec
DELETE FROM archive.oauth_client WHERE provider = $1 AND client = $2;

-- name: SetConnectionToken :exec
UPDATE archive.connection SET token = $2, error = '', updated_at = now() WHERE id = $1;

-- name: SetConnectionError :exec
UPDATE archive.connection SET error = $2, updated_at = now() WHERE id = $1 AND error <> $2;

-- name: DeleteConnection :one
DELETE FROM archive.connection WHERE id = $1 RETURNING *;

-- name: SetConnectionAccess :exec
WITH r AS (UPDATE archive.record x SET access = sqlc.narg(access)::text WHERE x.connection = sqlc.arg(conn)::uuid)
UPDATE archive.connection c SET access = sqlc.narg(access)::text, updated_at = now() WHERE c.id = sqlc.arg(conn)::uuid AND c.integration;

-- Connections from before integrations: the first of each integration's becomes the archive's, unless it has one.
-- name: UpgradeIntegrations :execrows
UPDATE archive.connection c SET integration = true
WHERE c.id IN (SELECT DISTINCT ON (x.provider) x.id FROM archive.connection x
    WHERE x.provider = ANY(sqlc.arg(providers)::text[]) AND NOT x.integration
      AND NOT EXISTS (SELECT FROM archive.connection y WHERE y.provider = x.provider AND y.integration)
    ORDER BY x.provider, x.created_at, x.id);

-- name: ClaimSync :one
UPDATE archive.connection SET lease_at = sqlc.arg(lease)::timestamptz
WHERE id = (SELECT c.id FROM archive.connection c WHERE c.sync_at <= now() AND (c.lease_at IS NULL OR c.lease_at < now())
    ORDER BY c.sync_at LIMIT 1 FOR UPDATE SKIP LOCKED)
RETURNING *;

-- name: NextSyncAt :one
SELECT greatest(sync_at, coalesce(lease_at, sync_at))::timestamptz FROM archive.connection WHERE sync_at IS NOT NULL ORDER BY 1 LIMIT 1;

-- name: SaveSync :exec
UPDATE archive.connection SET cursor = sqlc.arg(cursor), sync_at = sqlc.narg(sync_at), synced_at = coalesce(sqlc.narg(synced_at), synced_at),
    error = sqlc.arg(error), revoked = sqlc.arg(revoked), lease_at = NULL
WHERE id = sqlc.arg(id);

-- name: AskSync :exec
UPDATE archive.connection SET cursor = $2, sync_at = now(), asked_at = now(), revoked = false WHERE id = $1;

-- A push asks for a sync: one already due, or backing off, keeps its time.
-- name: WantSync :one
UPDATE archive.connection SET asked_at = now(), sync_at = coalesce(sync_at, now()) WHERE id = $1 AND NOT revoked RETURNING id;

-- name: CatchUpSyncs :many
UPDATE archive.connection SET asked_at = now(), sync_at = now()
WHERE (member = sqlc.arg(member) AND NOT integration OR integration) AND sync_at IS NULL AND NOT revoked AND (synced_at IS NULL OR synced_at < sqlc.arg(stale)::timestamptz)
RETURNING id;

-- name: WatchesFor :many
SELECT workspace, connection FROM archive.watch WHERE provider = $1 AND key = $2;

-- name: WatchSecret :one
SELECT secret FROM archive.watch WHERE provider = $1 AND key = $2 AND secret IS NOT NULL LIMIT 1;

-- name: ConnectionWatches :many
SELECT * FROM archive.watch WHERE connection = $1 ORDER BY key;

-- name: SaveWatch :exec
INSERT INTO archive.watch (provider, key, workspace, connection, resource, secret, expires_at)
VALUES (sqlc.arg(provider), sqlc.arg(key), sqlc.arg(workspace), sqlc.arg(connection), sqlc.arg(resource), sqlc.narg(secret), sqlc.narg(expires_at))
ON CONFLICT (provider, key, connection) DO UPDATE SET resource = EXCLUDED.resource, secret = coalesce(EXCLUDED.secret, archive.watch.secret),
    expires_at = EXCLUDED.expires_at;

-- A handshake's secret, kept by the key in the URL only it was sent: the first one wins.
-- name: KeepWatchSecret :execrows
INSERT INTO archive.watch (provider, key, workspace, connection, secret) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING;

-- name: DropWatches :exec
DELETE FROM archive.watch WHERE connection = sqlc.arg(connection) AND NOT key = ANY(sqlc.arg(keep)::text[]);

-- name: TryLock :one
SELECT pg_try_advisory_xact_lock(hashtextextended(sqlc.arg(name)::text, 0));

-- name: SaveRecords :many
INSERT INTO archive.record (connection, stream, source_id, data, updated_at, access)
SELECT sqlc.arg(connection), sqlc.arg(stream), r.id, r.data, coalesce(r.updated, now()), (SELECT c.access FROM archive.connection c WHERE c.id = sqlc.arg(connection))
FROM jsonb_to_recordset(sqlc.arg(records)::jsonb) AS r(id text, data jsonb, updated timestamptz)
ON CONFLICT (connection, stream, source_id) DO UPDATE SET data = EXCLUDED.data, deleted = false, seen_at = now(), access = EXCLUDED.access,
    updated_at = CASE WHEN archive.record.deleted OR archive.record.data IS DISTINCT FROM EXCLUDED.data THEN EXCLUDED.updated_at ELSE archive.record.updated_at END
RETURNING id, source_id;

-- name: SeeRecords :exec
UPDATE archive.record SET seen_at = now()
WHERE connection = sqlc.arg(connection) AND stream = sqlc.arg(stream) AND source_id = ANY(sqlc.arg(ids)::text[]) AND NOT deleted;

-- name: DeleteRecords :many
UPDATE archive.record SET deleted = true, updated_at = now(), seen_at = now()
WHERE connection = sqlc.arg(connection) AND stream = sqlc.arg(stream) AND source_id = ANY(sqlc.arg(ids)::text[]) AND NOT deleted
RETURNING id;

-- name: DeleteUnseenRecords :many
UPDATE archive.record SET deleted = true, updated_at = now()
WHERE connection = sqlc.arg(connection) AND stream = sqlc.arg(stream) AND seen_at < sqlc.arg(since)::timestamptz AND NOT deleted
RETURNING id;

-- name: DeleteStreamRecords :many
UPDATE archive.record SET deleted = true, updated_at = now()
WHERE connection = sqlc.arg(connection) AND stream = sqlc.arg(stream) AND NOT deleted
RETURNING id;

-- The records of a stream from after, by source_id, that are not deleted, to write into Archive's tables.
-- name: StreamRecords :many
SELECT id, source_id, data FROM archive.record
WHERE connection = sqlc.arg(connection) AND stream = sqlc.arg(stream) AND source_id > sqlc.arg(after)::text AND NOT deleted
ORDER BY source_id LIMIT sqlc.arg(n);

-- name: SaveSyncCursor :exec
UPDATE archive.connection SET cursor = $2 WHERE id = $1;

-- What records are in Archive's tables (tables.go): each statement writes one table from rows, a JSON array.

-- name: ClearRecordRows :exec
WITH m AS (DELETE FROM archive.mail_message x WHERE x.record = ANY(sqlc.arg(ids)::uuid[])),
    d AS (DELETE FROM archive.doc_page x WHERE x.record = ANY(sqlc.arg(ids)::uuid[])),
    c AS (DELETE FROM archive.payroll_company x WHERE x.record = ANY(sqlc.arg(ids)::uuid[])),
    w AS (DELETE FROM archive.payroll_worker x WHERE x.record = ANY(sqlc.arg(ids)::uuid[])),
    r AS (DELETE FROM archive.payroll_run x WHERE x.record = ANY(sqlc.arg(ids)::uuid[]))
DELETE FROM archive.payroll_pay x WHERE x.record = ANY(sqlc.arg(ids)::uuid[]);

-- People and organizations nothing names any more go.
-- name: DeleteUnknown :exec
WITH p AS (DELETE FROM archive.known_person k WHERE NOT EXISTS (SELECT FROM archive.mail_party x WHERE x.person = k.id)
    AND NOT EXISTS (SELECT FROM archive.payroll_worker x WHERE x.person = k.id))
DELETE FROM archive.known_org o WHERE NOT EXISTS (SELECT FROM archive.payroll_company x WHERE x.org = o.id);

-- name: SaveKnownPeople :exec
INSERT INTO archive.known_person (email) SELECT DISTINCT e FROM unnest(sqlc.arg(emails)::text[]) e ORDER BY e ON CONFLICT (email) DO NOTHING;

-- name: SaveKnownOrgs :exec
INSERT INTO archive.known_org (ein) SELECT DISTINCT e FROM unnest(sqlc.arg(eins)::text[]) e ORDER BY e ON CONFLICT (ein) DO NOTHING;

-- name: SaveMail :many
INSERT INTO archive.mail_message (record, connection, thread, subject, snippet, sent_at, labels, attachments)
SELECT r.record, sqlc.arg(connection), r.thread, r.subject, r.snippet, r.sent_at, coalesce(r.labels, '{}'), r.attachments
FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS r(record uuid, thread text, subject text, snippet text, sent_at timestamptz, labels text[], attachments int)
ON CONFLICT (record) DO UPDATE SET thread = EXCLUDED.thread, subject = EXCLUDED.subject, snippet = EXCLUDED.snippet, sent_at = EXCLUDED.sent_at,
    labels = EXCLUDED.labels, attachments = EXCLUDED.attachments
RETURNING id, record;

-- The parties of each of messages are those of batch, and only those.
-- name: SaveMailParties :exec
WITH gone AS (DELETE FROM archive.mail_party p WHERE p.message = ANY(sqlc.arg(messages)::uuid[])
    AND NOT EXISTS (SELECT FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS g(message uuid, email text, role text) JOIN archive.known_person k ON k.email = g.email
        WHERE g.message = p.message AND g.role = p.role AND k.id = p.person))
INSERT INTO archive.mail_party (message, person, role, name)
SELECT DISTINCT ON (r.message, r.role, k.id) r.message, k.id, r.role, CASE WHEN r.name <> '' THEN ROW(r.name, NULL, NULL, NULL)::archive.name END
FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS r(message uuid, email text, role text, name text) JOIN archive.known_person k ON k.email = r.email
ON CONFLICT (message, role, person) DO UPDATE SET name = EXCLUDED.name;

-- name: SaveDocPages :exec
INSERT INTO archive.doc_page (record, connection, kind, title, url, icon, edited_at)
SELECT r.record, sqlc.arg(connection), coalesce(nullif(r.kind, ''), 'page'), r.title, nullif(r.url, ''), r.icon, r.edited_at
FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS r(record uuid, kind text, title text, url text, icon text, edited_at timestamptz)
ON CONFLICT (record) DO UPDATE SET kind = EXCLUDED.kind, title = EXCLUDED.title, url = EXCLUDED.url, icon = EXCLUDED.icon, edited_at = EXCLUDED.edited_at;

-- A company written links the workers and payrolls of the connection that came before it.
-- name: SavePayrollCompanies :exec
WITH c AS (
    INSERT INTO archive.payroll_company (record, connection, source_id, org, name, trade_name, entity_type)
    SELECT r.record, sqlc.arg(connection), r.source_id, (SELECT o.id FROM archive.known_org o WHERE o.ein = r.ein), r.name, r.trade_name, r.entity_type
    FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS r(record uuid, source_id text, ein text, name text, trade_name text, entity_type text)
    ON CONFLICT (record) DO UPDATE SET source_id = EXCLUDED.source_id, org = EXCLUDED.org, name = EXCLUDED.name, trade_name = EXCLUDED.trade_name,
        entity_type = EXCLUDED.entity_type
    RETURNING id, source_id
), w AS (UPDATE archive.payroll_worker x SET company = c.id FROM c WHERE x.connection = sqlc.arg(connection) AND x.company_source = c.source_id AND x.company IS DISTINCT FROM c.id)
UPDATE archive.payroll_run x SET company = c.id FROM c WHERE x.connection = sqlc.arg(connection) AND x.company_source = c.source_id AND x.company IS DISTINCT FROM c.id;

-- A worker written links the pays of the connection that came before them.
-- name: SavePayrollWorkers :exec
WITH w AS (
    INSERT INTO archive.payroll_worker (record, connection, source_id, kind, company, company_source, person, name, business_name, email, personal_email,
        title, hired_on, terminated_on, active, pay_rate, pay_per)
    SELECT r.record, sqlc.arg(connection), r.source_id, r.kind,
        (SELECT c.id FROM archive.payroll_company c WHERE c.connection = sqlc.arg(connection) AND c.source_id = r.company_source), coalesce(r.company_source, ''),
        (SELECT k.id FROM archive.known_person k WHERE k.email = r.email),
        CASE WHEN r.full_name <> '' THEN ROW(r.full_name, nullif(r.given, ''), nullif(r.family, ''), NULL)::archive.name END, coalesce(r.business_name, ''),
        nullif(r.email, ''), nullif(r.personal_email, ''), coalesce(r.title, ''), r.hired_on, r.terminated_on, r.active,
        archive.money_of(r.rate, r.currency), coalesce(r.pay_per, '')
    FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS r(record uuid, source_id text, kind text, company_source text, email text, personal_email text,
        full_name text, given text, family text, business_name text, title text, hired_on date, terminated_on date, active bool, rate bigint, currency text, pay_per text)
    ON CONFLICT (record) DO UPDATE SET source_id = EXCLUDED.source_id, kind = EXCLUDED.kind, company = EXCLUDED.company, company_source = EXCLUDED.company_source,
        person = EXCLUDED.person, name = EXCLUDED.name, business_name = EXCLUDED.business_name, email = EXCLUDED.email, personal_email = EXCLUDED.personal_email,
        title = EXCLUDED.title, hired_on = EXCLUDED.hired_on, terminated_on = EXCLUDED.terminated_on, active = EXCLUDED.active, pay_rate = EXCLUDED.pay_rate,
        pay_per = EXCLUDED.pay_per
    RETURNING id, source_id
)
UPDATE archive.payroll_pay x SET worker = w.id FROM w WHERE x.connection = sqlc.arg(connection) AND x.worker_source = w.source_id AND x.worker IS DISTINCT FROM w.id;

-- name: SavePayrollRuns :exec
INSERT INTO archive.payroll_run (record, connection, source_id, company, company_source, period_start, period_end, check_date, processed, off_cycle, external,
    gross_pay, net_pay, employee_taxes, employer_taxes)
SELECT r.record, sqlc.arg(connection), r.source_id, (SELECT c.id FROM archive.payroll_company c WHERE c.connection = sqlc.arg(connection) AND c.source_id = r.company_source),
    coalesce(r.company_source, ''), r.period_start, r.period_end, r.check_date, r.processed, r.off_cycle, r.external,
    archive.money_of(r.gross, r.currency), archive.money_of(r.net, r.currency), archive.money_of(r.taxes, r.currency), archive.money_of(r.employer_taxes, r.currency)
FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS r(record uuid, source_id text, company_source text, period_start date, period_end date, check_date date,
    processed bool, off_cycle bool, external bool, gross bigint, net bigint, taxes bigint, employer_taxes bigint, currency text)
ON CONFLICT (record) DO UPDATE SET source_id = EXCLUDED.source_id, company = EXCLUDED.company, company_source = EXCLUDED.company_source,
    period_start = EXCLUDED.period_start, period_end = EXCLUDED.period_end, check_date = EXCLUDED.check_date, processed = EXCLUDED.processed,
    off_cycle = EXCLUDED.off_cycle, external = EXCLUDED.external, gross_pay = EXCLUDED.gross_pay, net_pay = EXCLUDED.net_pay,
    employee_taxes = EXCLUDED.employee_taxes, employer_taxes = EXCLUDED.employer_taxes;

-- The pays of each of records are those of batch, and only those.
-- name: SavePayrollPays :exec
WITH gone AS (DELETE FROM archive.payroll_pay p WHERE p.record = ANY(sqlc.arg(records)::uuid[])
    AND NOT EXISTS (SELECT FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS g(record uuid, worker_source text) WHERE g.record = p.record AND g.worker_source = p.worker_source))
INSERT INTO archive.payroll_pay (record, connection, run, worker, worker_source, paid_on, gross_pay, net_pay, deductions, employee_taxes, employer_taxes, payment_method)
SELECT r.record, sqlc.arg(connection), (SELECT u.id FROM archive.payroll_run u WHERE u.record = r.record),
    (SELECT w.id FROM archive.payroll_worker w WHERE w.connection = sqlc.arg(connection) AND w.source_id = r.worker_source), r.worker_source, r.paid_on,
    archive.money_of(r.gross, r.currency), archive.money_of(r.net, r.currency), archive.money_of(r.deductions, r.currency), archive.money_of(r.taxes, r.currency),
    archive.money_of(r.employer_taxes, r.currency), coalesce(r.payment_method, '')
FROM jsonb_to_recordset(sqlc.arg(batch)::jsonb) AS r(record uuid, worker_source text, paid_on date, gross bigint, net bigint,
    deductions bigint, taxes bigint, employer_taxes bigint, currency text, payment_method text)
ON CONFLICT (record, worker_source) DO UPDATE SET run = EXCLUDED.run, worker = EXCLUDED.worker, paid_on = EXCLUDED.paid_on, gross_pay = EXCLUDED.gross_pay,
    net_pay = EXCLUDED.net_pay, deductions = EXCLUDED.deductions, employee_taxes = EXCLUDED.employee_taxes, employer_taxes = EXCLUDED.employer_taxes,
    payment_method = EXCLUDED.payment_method;
