CREATE SCHEMA archive;

CREATE TABLE archive.space (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL UNIQUE,
    role       text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive."user" (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    role       text NOT NULL UNIQUE,
    home       uuid REFERENCES archive.space,
    signin     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.token (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES archive."user" ON DELETE CASCADE,
    kind       text NOT NULL,
    name       text NOT NULL DEFAULT '',
    role       text UNIQUE,
    hash       text NOT NULL UNIQUE,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX token_user_idx ON archive.token (user_id);

CREATE TABLE archive.revision (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    hash       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
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
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    search     tsvector GENERATED ALWAYS AS (setweight(archive.search_doc(name), 'A') || setweight(archive.search_doc(text), 'C')) STORED,
    UNIQUE (space, hash, name)
);
CREATE INDEX file_pending_idx ON archive.file (id) WHERE status = 'pending';
CREATE INDEX file_search_idx ON archive.file USING gin (search);

CREATE TABLE archive.upload (
    id         uuid PRIMARY KEY,
    role       text NOT NULL,
    space      uuid NOT NULL REFERENCES archive.space ON DELETE CASCADE,
    name       text NOT NULL,
    length     bigint NOT NULL,
    parts      bigint[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.view (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL,
    space      uuid NOT NULL REFERENCES archive.space,
    type       text NOT NULL,
    layout     text NOT NULL DEFAULT 'table',
    by_field   text NOT NULL DEFAULT '',
    filter     jsonb NOT NULL DEFAULT '{}',
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (space, name)
);

CREATE TABLE archive.secret (
    id          uuid PRIMARY KEY,
    table_name  text NOT NULL,
    row_id      uuid NOT NULL,
    column_name text NOT NULL,
    type        text NOT NULL,
    value       bytea NOT NULL,
    space       uuid NOT NULL REFERENCES archive.space,
    UNIQUE (table_name, row_id, column_name)
);
CREATE INDEX secret_space_idx ON archive.secret (space);

CREATE FUNCTION archive.my_spaces(lvl text DEFAULT 'viewer') RETURNS uuid[] LANGUAGE sql STABLE AS $$
    SELECT coalesce(array_agg(id), '{}') FROM archive.space WHERE pg_has_role(current_user, (role || '_' || lvl)::name, 'USAGE')
$$;

CREATE FUNCTION archive.ontology_admin() RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT pg_has_role(current_user, '{p}_admin', 'USAGE') OR cardinality(archive.my_spaces('admin')) > 0
$$;

CREATE FUNCTION archive.secret_granted(typ text) RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_secret_' || typ AND pg_has_role(current_user, oid, 'USAGE'))
$$;

CREATE FUNCTION archive.secret_clear(tbl text, rid uuid, col text) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
    DELETE FROM archive.secret s USING archive.space sp
    WHERE s.table_name = tbl AND s.row_id = rid AND (col IS NULL OR s.column_name = col) AND sp.id = s.space
      AND pg_has_role(current_setting('role')::name, (sp.role || '_editor')::name, 'USAGE')
$$;

GRANT USAGE ON SCHEMA archive TO {p}_user;
GRANT SELECT ON archive.space TO {p}_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON archive.file, archive.view, archive.secret TO {p}_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {p}_user;

ALTER TABLE archive.space ENABLE ROW LEVEL SECURITY;
ALTER TABLE archive.space FORCE ROW LEVEL SECURITY;
CREATE POLICY member ON archive.space FOR SELECT USING (pg_has_role(current_user, (role || '_viewer')::name, 'USAGE'));

ALTER TABLE archive.file ENABLE ROW LEVEL SECURITY;
ALTER TABLE archive.file FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON archive.file FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON archive.file USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));

ALTER TABLE archive.view ENABLE ROW LEVEL SECURITY;
ALTER TABLE archive.view FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON archive.view FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON archive.view USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));

ALTER TABLE archive.secret ENABLE ROW LEVEL SECURITY;
ALTER TABLE archive.secret FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON archive.secret FOR SELECT USING (space = ANY (archive.my_spaces()) AND archive.secret_granted(type));
CREATE POLICY editor_insert ON archive.secret FOR INSERT WITH CHECK (space = ANY (archive.my_spaces('editor')));
CREATE POLICY editor_update ON archive.secret FOR UPDATE USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
CREATE POLICY editor_delete ON archive.secret FOR DELETE USING (space = ANY (archive.my_spaces('editor')));

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
CREATE DOMAIN archive.iban AS text CHECK (VALUE ~ '^((NO)[0-9]{2}[A-Z0-9]{11}|(BE)[0-9]{2}[A-Z0-9]{12}|(DK|FI|FK|FO|GL|NL|SD)[0-9]{2}[A-Z0-9]{14}|(MK|SI)[0-9]{2}[A-Z0-9]{15}|(AT|BA|EE|KZ|LT|LU|MN|XK)[0-9]{2}[A-Z0-9]{16}|(CH|HR|LI|LV)[0-9]{2}[A-Z0-9]{17}|(BG|BH|CR|DE|GB|GE|IE|ME|RS|VA)[0-9]{2}[A-Z0-9]{18}|(AE|GI|IL|IQ|OM|SO|TL)[0-9]{2}[A-Z0-9]{19}|(AD|CZ|ES|MD|PK|RO|SA|SE|SK|TN|VG)[0-9]{2}[A-Z0-9]{20}|(LY|PT|ST)[0-9]{2}[A-Z0-9]{21}|(IS|TR)[0-9]{2}[A-Z0-9]{22}|(BI|DJ|FR|GR|IT|MC|MR|SM)[0-9]{2}[A-Z0-9]{23}|(AL|AZ|BY|CY|DO|GT|HU|LB|NI|PL|SV)[0-9]{2}[A-Z0-9]{24}|(BR|EG|PS|QA|UA)[0-9]{2}[A-Z0-9]{25}|(JO|KW|MU|YE)[0-9]{2}[A-Z0-9]{26}|(MT|SC)[0-9]{2}[A-Z0-9]{27}|(LC)[0-9]{2}[A-Z0-9]{28}|(RU)[0-9]{2}[A-Z0-9]{29})$');
COMMENT ON DOMAIN archive.iban IS 'ISO 13616 bank account number';
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
CREATE DOMAIN archive.vat_id AS text CHECK (VALUE ~ '^(ATU[0-9]{8}|BE[01][0-9]{9}|BG[0-9]{9,10}|CY[0-9]{8}[A-Z]|CZ[0-9]{8,10}|DE[0-9]{9}|DK[0-9]{8}|EE[0-9]{9}|EL[0-9]{9}|ES[A-Z0-9][0-9]{7}[A-Z0-9]|FI[0-9]{8}|FR[A-HJ-NP-Z0-9]{2}[0-9]{9}|HR[0-9]{11}|HU[0-9]{8}|IE([0-9]{7}[A-W][A-I]?|[0-9][A-Z+*][0-9]{5}[A-W])|IT[0-9]{11}|LT([0-9]{9}|[0-9]{12})|LU[0-9]{8}|LV[0-9]{11}|MT[0-9]{8}|NL[0-9]{9}B[0-9]{2}|PL[0-9]{10}|PT[0-9]{9}|RO[0-9]{2,10}|SE[0-9]{12}|SI[0-9]{8}|SK[0-9]{10}|(GB|XI)([0-9]{9}|[0-9]{12}|GD[0-9]{3}|HA[0-9]{3}))$');
COMMENT ON DOMAIN archive.vat_id IS 'EU or UK VAT number with its country prefix';
CREATE DOMAIN archive.us_ein AS text CHECK (VALUE ~ '^(0[1-6]|1[0-6]|2[0-7]|3[0-9]|4[0-8]|5[0-9]|6[0-8]|7[1-7]|8[0-8]|9[0-58-9])-[0-9]{7}$');
COMMENT ON DOMAIN archive.us_ein IS 'US employer identification number';
CREATE DOMAIN archive.us_ssn AS text;
COMMENT ON DOMAIN archive.us_ssn IS 'US social security number, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.nl_bsn AS text;
COMMENT ON DOMAIN archive.nl_bsn IS 'Dutch citizen service number, stored encrypted; the column holds its mask';
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
