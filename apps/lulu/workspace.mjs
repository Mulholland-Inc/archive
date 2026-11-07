import { createHash } from 'node:crypto';

const API = process.env.API || 'http://localhost:8091';
const headers = { 'Content-Type': 'application/json', ...(process.env.BEARER ? { Authorization: `Bearer ${process.env.BEARER}` } : {}), ...(process.env.WORKSPACE ? { 'Archive-Workspace': process.env.WORKSPACE } : {}) };

const run = async code => {
  const res = await fetch(`${API}/api/run`, { method: 'POST', headers, body: JSON.stringify({ code }) });
  const out = await res.json();
  if (!res.ok || out.error) throw new Error(`${res.status} ${String(out.error || JSON.stringify(out)).slice(0, 1500)}`);
  return out.result;
};

const migrate = async (name, sql) => {
  const out = await run(`const m = await archive.migrate(${JSON.stringify(sql)}, { name: ${JSON.stringify(name)} }); if (!m.confirm) return { skipped: true }; const a = await archive.migrate(${JSON.stringify(sql)}, { name: ${JSON.stringify(name)}, confirm: m.confirm }); return { applied: a.applied, breaks: a.breaks };`);
  console.log(name, out.skipped ? 'skipped' : 'applied');
};

const versioned = async (name, sql) => migrate(`${name}_${createHash('sha256').update(sql).digest('hex').slice(0, 12)}`, sql);

const SCHEMA = String.raw`
CREATE TABLE intake (brand text NOT NULL DEFAULT '', url text NOT NULL DEFAULT '', support text NOT NULL DEFAULT '', sections text[] NOT NULL DEFAULT '{}') INHERITS (archive.object);
COMMENT ON TABLE intake IS 'One row: who this desk is, the portal''s address and the order sections read in.';
COMMENT ON COLUMN intake.brand IS 'The name every mail and message wears.';
COMMENT ON COLUMN intake.url IS 'The portal''s origin, what vendor links are minted under.';
COMMENT ON COLUMN intake.support IS 'Where a vendor''s reply lands.';
COMMENT ON COLUMN intake.sections IS 'The order a sheet''s sections read in; one not listed reads last.';

CREATE TABLE vendor (slug text NOT NULL DEFAULT '', country text NOT NULL DEFAULT '', contacts jsonb NOT NULL DEFAULT '[]', photo_map jsonb NOT NULL DEFAULT '{}', member uuid, link_at timestamptz,
  CONSTRAINT vendor_contacts_array CHECK (jsonb_typeof(contacts) = 'array'), CONSTRAINT vendor_photo_map_object CHECK (jsonb_typeof(photo_map) = 'object')) INHERITS (archive.object);
COMMENT ON TABLE vendor IS 'A vendor the buying team sends builds to: its name and the contacts remembered for it.';
COMMENT ON COLUMN vendor.slug IS 'Its address, derived from the name.';
COMMENT ON COLUMN vendor.country IS 'Where the vendor is, when known.';
COMMENT ON COLUMN vendor.contacts IS 'The contacts saved for this vendor, [{name, email}]: what a new build''s recipients are prefilled with.';
COMMENT ON COLUMN vendor.photo_map IS 'What the patterns in this vendor''s photo file names mean, {pattern: angle}.';
COMMENT ON COLUMN vendor.member IS 'The guest member that holds this vendor''s link, made the first time it is minted.';
COMMENT ON COLUMN vendor.link_at IS 'When its link was last minted.';
CREATE UNIQUE INDEX vendor_slug_key ON vendor (slug);
CREATE UNIQUE INDEX vendor_name_key ON vendor (lower(name));

CREATE TABLE template (key text NOT NULL DEFAULT '', parent uuid, icon text NOT NULL DEFAULT '', sheet text NOT NULL DEFAULT 'VENDOR FILL', shell uuid,
  hides text[] NOT NULL DEFAULT '{}', images jsonb, photo jsonb, export jsonb NOT NULL DEFAULT '{}') INHERITS (archive.object);
ALTER TABLE template ADD CONSTRAINT template_parent_fkey FOREIGN KEY (parent) REFERENCES template ON DELETE RESTRICT;
COMMENT ON TABLE template IS 'A category template: what a vendor is asked for one kind of product, layered over its parent, and the workbook an approved build exports as.';
COMMENT ON COLUMN template.key IS 'Its address, derived from the name and never changed.';
COMMENT ON COLUMN template.parent IS 'The template this one inherits from; empty only for the root.';
COMMENT ON COLUMN template.icon IS 'The icon it wears; empty is the default for its key.';
COMMENT ON COLUMN template.sheet IS 'The name of the fill tab.';
COMMENT ON COLUMN template.shell IS 'The workbook shell the export fills.';
COMMENT ON COLUMN template.hides IS 'Parent fields this template no longer asks.';
COMMENT ON COLUMN template.images IS 'The angles owed, [{role, min, max, when}]; null inherits.';
COMMENT ON COLUMN template.photo IS 'The technical spec every photograph is held to; null inherits.';
COMMENT ON COLUMN template.export IS 'Where answers land in the shell: {row, columns, compose}.';
CREATE UNIQUE INDEX template_key_key ON template (key) WHERE key <> '';

CREATE TABLE field (template uuid NOT NULL REFERENCES template ON DELETE CASCADE, key text NOT NULL, section text NOT NULL DEFAULT '', type text NOT NULL DEFAULT 'text', owner text NOT NULL DEFAULT 'vendor', required boolean NOT NULL DEFAULT false,
  required_when jsonb, must jsonb, fills jsonb, options text[], parts text[], help text, note text, multi boolean NOT NULL DEFAULT false, pct_breakdown boolean NOT NULL DEFAULT false, position integer NOT NULL DEFAULT 0,
  CONSTRAINT field_type CHECK (type IN ('text', 'number', 'select', 'yesno', 'country', 'dimensions')), CONSTRAINT field_owner CHECK (owner IN ('vendor', 'lg'))) INHERITS (archive.object);
COMMENT ON TABLE field IS 'What one template adds or changes over its parent for one field. name is its label.';
COMMENT ON COLUMN field.required_when IS 'Conditions any one of which makes it required: a list of lists of {field, op, values, measure}.';
COMMENT ON COLUMN field.must IS 'What an answer has to be: [{when, op, values, message}].';
COMMENT ON COLUMN field.fills IS 'How a field fills itself: [{when, value}], the first that holds.';
CREATE UNIQUE INDEX field_template_key ON field (template, key);

CREATE TABLE build (number integer NOT NULL DEFAULT 0, created_by text NOT NULL DEFAULT '', created_by_name text NOT NULL DEFAULT '', vendor uuid NOT NULL REFERENCES vendor ON DELETE RESTRICT, emails text[] NOT NULL DEFAULT '{}', template uuid REFERENCES template ON DELETE RESTRICT,
  stage text NOT NULL DEFAULT 'draft', due_at timestamptz, remind_every_days integer NOT NULL DEFAULT 3, reminders_sent integer NOT NULL DEFAULT 0, last_reminded_at timestamptz, sent_at timestamptz, opened_at timestamptz, submitted_at timestamptz, returned_at timestamptz,
  return_note text NOT NULL DEFAULT '', approved_at timestamptz, cancelled_at timestamptz, source text NOT NULL DEFAULT '', mapping jsonb NOT NULL DEFAULT '{}', columns text[] NOT NULL DEFAULT '{}', column_map jsonb NOT NULL DEFAULT '{}',
  skus integer NOT NULL DEFAULT 0, clean integer NOT NULL DEFAULT 0, errors integer NOT NULL DEFAULT 0, warns integer NOT NULL DEFAULT 0, sorting integer NOT NULL DEFAULT 0, photos boolean NOT NULL DEFAULT true, build_type text NOT NULL DEFAULT 'Dropship', restamp_at timestamptz, reset_at timestamptz,
  CONSTRAINT build_name_present CHECK (btrim(name) <> ''), CONSTRAINT build_stage CHECK (stage IN ('draft', 'sent', 'in_progress', 'review', 'complete', 'cancelled')), CONSTRAINT build_remind_every CHECK (remind_every_days >= 1), CONSTRAINT build_type_known CHECK (build_type IN ('Owned', 'Dropship', 'MTO')),
  CONSTRAINT build_counts CHECK (skus >= 0 AND clean >= 0 AND errors >= 0 AND warns >= 0 AND clean <= skus)) INHERITS (archive.object);
COMMENT ON TABLE build IS 'One vendor asked for one category''s data on a list of SKUs: what was asked, who it went to, and where it stands from draft to approved.';
COMMENT ON COLUMN build.number IS 'What people call it, counted per workspace; set on insert.';
COMMENT ON COLUMN build.stage IS 'draft, sent, in_progress, review, complete or cancelled.';
COMMENT ON COLUMN build.restamp_at IS 'Written to stamp every row of the build again.';
COMMENT ON COLUMN build.reset_at IS 'Written to put every row back to what it was created with.';
CREATE UNIQUE INDEX build_number_key ON build (number);
CREATE INDEX build_vendor_idx ON build (vendor, number);

CREATE TABLE build_row (build uuid NOT NULL REFERENCES build ON DELETE CASCADE, sku text NOT NULL DEFAULT '', position integer NOT NULL DEFAULT 0, status text NOT NULL DEFAULT 'fix', seeded jsonb NOT NULL DEFAULT '{}', lg_seeded jsonb NOT NULL DEFAULT '{}', defaults jsonb NOT NULL DEFAULT '{}', lg_defaults jsonb NOT NULL DEFAULT '{}', listed text[] NOT NULL DEFAULT '{}',
  issues jsonb NOT NULL DEFAULT '[]', errors integer NOT NULL DEFAULT 0, warns integer NOT NULL DEFAULT 0, CONSTRAINT build_row_status CHECK (status IN ('fix', 'note', 'ok'))) INHERITS (archive.object);
COMMENT ON TABLE build_row IS 'One product of a build, by its SKU, its name or both. Every text column besides sku and status is a field, named by its key; its comment is its label, then section, type, owner and templates lines.';
COMMENT ON COLUMN build_row.sku IS 'The manufacturer SKU as stamped; empty while the product has none.';
COMMENT ON COLUMN build_row.status IS 'fix while an error is open, note while only warnings are, ok when clean.';
COMMENT ON COLUMN build_row.seeded IS 'The vendor-owned values it was created with: what Reset puts back.';
COMMENT ON COLUMN build_row.lg_seeded IS 'The buyer-owned values it was created with.';
COMMENT ON COLUMN build_row.defaults IS 'Vendor-owned values filled from a rule and not changed since, {key: value}.';
COMMENT ON COLUMN build_row.lg_defaults IS 'Buyer-owned values filled from a rule and not changed since.';
COMMENT ON COLUMN build_row.listed IS 'The field keys the buyer listed this product by, which a vendor cannot change.';
COMMENT ON COLUMN build_row.issues IS 'What validation found, [{row, field, level, message}], in field order.';
CREATE INDEX build_row_position_idx ON build_row (build, position);
CREATE UNIQUE INDEX build_row_sku_unique ON build_row (build, lower(btrim(sku))) WHERE btrim(sku) <> '';
CREATE UNIQUE INDEX build_row_name_unique ON build_row (build, lower(btrim(name))) WHERE btrim(sku) = '';
CREATE INDEX build_row_sku_key_idx ON build_row (lower(btrim(sku)));
CREATE INDEX build_row_name_key_idx ON build_row (lower(btrim(name)));

CREATE TABLE build_image (build uuid NOT NULL REFERENCES build ON DELETE CASCADE, file uuid NOT NULL, build_row uuid REFERENCES build_row ON DELETE CASCADE, role text NOT NULL DEFAULT '', position integer NOT NULL DEFAULT 0, media text NOT NULL DEFAULT '',
  width integer NOT NULL DEFAULT 0, height integer NOT NULL DEFAULT 0, transparent boolean NOT NULL DEFAULT false, fill real NOT NULL DEFAULT 0, measured boolean NOT NULL DEFAULT false) INHERITS (archive.object);
COMMENT ON TABLE build_image IS 'A photograph uploaded to a build and the product and angle it was placed on, if any.';
COMMENT ON COLUMN build_image.role IS 'The angle it fills for that product; empty while unplaced.';
COMMENT ON COLUMN build_image.measured IS 'Whether its bytes have been read; one that has not is held to no photo spec.';
CREATE INDEX build_image_build_idx ON build_image (build, position);
CREATE INDEX build_image_row_idx ON build_image (build_row);

CREATE TABLE build_activity (build uuid NOT NULL REFERENCES build ON DELETE CASCADE, at timestamptz NOT NULL DEFAULT now(), who text NOT NULL DEFAULT 'system', text text NOT NULL DEFAULT '', CONSTRAINT build_activity_who CHECK (who IN ('system', 'vendor', 'ba'))) INHERITS (archive.object);
COMMENT ON TABLE build_activity IS 'One line of a build''s history: what happened, when, and whether the app, the vendor or the buying assistant did it.';
CREATE INDEX build_activity_build_idx ON build_activity (build, at);

CREATE TABLE cell_answer (field text NOT NULL, raw text NOT NULL, answers boolean NOT NULL, question uuid, decided_at timestamptz NOT NULL DEFAULT now(), CONSTRAINT cell_answer_field_present CHECK (btrim(field) <> ''), CONSTRAINT cell_answer_raw_present CHECK (btrim(raw) <> '')) INHERITS (archive.object);
COMMENT ON TABLE cell_answer IS 'Whether a word a vendor typed into a field answers it, as a model judged it once: a word that does not is a blank to validation.';
CREATE UNIQUE INDEX cell_answer_word_key ON cell_answer (field, lower(btrim(raw)));

CREATE TABLE vendor_answer (vendor uuid NOT NULL REFERENCES vendor ON DELETE CASCADE, kind text NOT NULL, field text NOT NULL DEFAULT '', raw text NOT NULL, value text NOT NULL DEFAULT '', decided_at timestamptz NOT NULL DEFAULT now(), CONSTRAINT vendor_answer_kind CHECK (kind IN ('header', 'cell')), CONSTRAINT vendor_answer_raw_present CHECK (btrim(raw) <> ''), CONSTRAINT vendor_answer_field_by_kind CHECK ((kind = 'cell') = (btrim(field) <> ''))) INHERITS (archive.object);
COMMENT ON TABLE vendor_answer IS 'One word of a vendor''s own, and what they said it means: a column heading read as a field, or a cell read as an option.';
CREATE UNIQUE INDEX vendor_answer_word_key ON vendor_answer (vendor, kind, field, lower(btrim(raw)));

CREATE TYPE lead_time_rule_unit AS ENUM ('business_days', 'days', 'weeks');
CREATE TABLE lead_time_rule (vendor uuid NOT NULL REFERENCES vendor ON DELETE CASCADE, ship_method text, min_time integer NOT NULL, max_time integer NOT NULL, unit lead_time_rule_unit NOT NULL DEFAULT 'weeks', note text NOT NULL DEFAULT '', CONSTRAINT lead_time_rule_range CHECK (min_time >= 0 AND max_time >= min_time), CONSTRAINT lead_time_rule_method_present CHECK (ship_method IS NULL OR btrim(ship_method) <> '')) INHERITS (archive.object);
COMMENT ON TABLE lead_time_rule IS 'How long a vendor''s product takes from the warehouse to the customer by one ship method, or by any.';
CREATE UNIQUE INDEX lead_time_rule_method_key ON lead_time_rule (vendor, lower(coalesce(ship_method, '')));

CREATE TABLE product (identifier text NOT NULL, category text NOT NULL, description text, CONSTRAINT product_identifier_present CHECK (btrim(identifier) <> ''), CONSTRAINT product_category_present CHECK (btrim(category) <> '')) INHERITS (archive.object);
COMMENT ON TABLE product IS 'A product captured from an approved build.';
CREATE UNIQUE INDEX product_identifier_key ON product (lower(identifier));

CREATE POLICY guest ON intake AS RESTRICTIVE USING (true);
CREATE POLICY guest ON vendor AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR id = (SELECT archive.scope())) WITH CHECK ((SELECT archive.scope()) IS NULL OR id = (SELECT archive.scope()));
CREATE POLICY guest ON template AS RESTRICTIVE USING (true);
CREATE POLICY guest ON field AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR owner <> 'lg' OR has_table_privilege('public.field', 'DELETE'));
CREATE POLICY guest ON build AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR (vendor = (SELECT archive.scope()) AND stage <> 'draft')) WITH CHECK ((SELECT archive.scope()) IS NULL OR (vendor = (SELECT archive.scope()) AND stage <> 'draft'));
CREATE POLICY guest ON build_row AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR build IN (SELECT b.id FROM build b)) WITH CHECK ((SELECT archive.scope()) IS NULL OR build IN (SELECT b.id FROM build b));
CREATE POLICY guest ON build_image AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR build IN (SELECT b.id FROM build b)) WITH CHECK ((SELECT archive.scope()) IS NULL OR build IN (SELECT b.id FROM build b));
CREATE POLICY guest ON build_activity AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR build IN (SELECT b.id FROM build b)) WITH CHECK ((SELECT archive.scope()) IS NULL OR build IN (SELECT b.id FROM build b));
CREATE POLICY guest ON vendor_answer AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR vendor = (SELECT archive.scope())) WITH CHECK ((SELECT archive.scope()) IS NULL OR vendor = (SELECT archive.scope()));
CREATE POLICY guest ON lead_time_rule AS RESTRICTIVE USING ((SELECT archive.scope()) IS NULL OR vendor = (SELECT archive.scope())) WITH CHECK ((SELECT archive.scope()) IS NULL);

GRANT SELECT (id, name, brand, support) ON intake TO guest;
GRANT SELECT (id, name, slug, country, created_at, updated_at) ON vendor TO guest;
GRANT SELECT (id, name, key, parent, icon, sheet, hides, images, photo, created_at, updated_at) ON template TO guest;
GRANT SELECT (id, name, template, key, section, type, owner, required, required_when, must, fills, options, parts, help, note, multi, pct_breakdown, position) ON field TO guest;
GRANT SELECT (id, name, number, vendor, template, stage, due_at, sent_at, opened_at, submitted_at, returned_at, return_note, approved_at, cancelled_at, source, skus, clean, errors, warns, sorting, photos, build_type, created_at, updated_at) ON build TO guest;
GRANT SELECT (id, name, build, sku, position, status, defaults, listed, issues, errors, warns, created_at, updated_at), UPDATE (updated_at) ON build_row TO guest;
GRANT SELECT (id, name, build, file, build_row, role, position, media, width, height, transparent, fill, measured, created_at, updated_at), UPDATE (build_row, role, position, updated_at), DELETE ON build_image TO guest;
GRANT SELECT (id, name, build, at, who, text, created_at) ON build_activity TO guest;
GRANT SELECT (id, name, vendor, kind, field, raw, value, decided_at, created_at) ON vendor_answer TO guest;
GRANT SELECT (id, name, vendor, ship_method, min_time, max_time, unit, note, created_at) ON lead_time_rule TO guest;
`;

const DEFINER = 'LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, archive, pg_catalog';

const FUNCTIONS = String.raw`
CREATE OR REPLACE FUNCTION go_trim(s text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT regexp_replace(s, '^[ \t\n\v\f\r\u0085   -     　]+|[ \t\n\v\f\r\u0085   -     　]+$', '', 'g')
$f$;

CREATE OR REPLACE FUNCTION multi_parts(v text) RETURNS text[] LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT coalesce(array_agg(p ORDER BY n) FILTER (WHERE p <> ''), '{}') FROM (SELECT go_trim(x) AS p, n FROM unnest(regexp_split_to_array(coalesce(v, ''), '\s*[;\n]\s*')) WITH ORDINALITY AS t(x, n)) q
$f$;

CREATE OR REPLACE FUNCTION js_number(raw text) RETURNS double precision LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE s text := go_trim(coalesce(raw, ''));
BEGIN
  IF s = '' THEN RETURN 0; END IF;
  IF s ~ '^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$' THEN
    IF length(s) < 300 AND s !~ '[eE]' THEN RETURN s::double precision; END IF;
    BEGIN
      RETURN s::double precision;
    EXCEPTION WHEN others THEN
      RETURN NULL;
    END;
  END IF;
  IF s ~ '^0[xX][0-9a-fA-F]+$' AND length(s) < 16 THEN RETURN ('x' || lpad(substr(s, 3), 16, '0'))::bit(64)::bigint::double precision; END IF;
  IF s ~ '^0[bB][01]+$' AND length(s) < 60 THEN RETURN (lpad(substr(s, 3), 64, '0'))::bit(64)::bigint::double precision; END IF;
  IF s ~ '^0[oO][0-7]+$' AND length(s) < 20 THEN
    RETURN (SELECT sum(substr(s, i, 1)::int * 8 ^ (length(s) - i)) FROM generate_series(3, length(s)) i);
  END IF;
  RETURN NULL;
END $f$;

CREATE OR REPLACE FUNCTION number_text(raw text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT regexp_replace(regexp_replace(go_trim(raw), '[$,\s]', '', 'g'), '(pounds|pound|lbs|lb|kilograms|kilogram|kgs|kg|grams|gram|g|ounces|ounce|oz|inches|inch|in|feet|foot|ft|centimetres|centimeters|centimetre|centimeter|cm|millimetres|millimeters|mm|metres|meters|metre|meter|m|"|'')$', '', 'i')
$f$;

CREATE OR REPLACE FUNCTION countries() RETURNS text[] LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT ARRAY['Afghanistan','Albania','Algeria','Andorra','Angola','Antigua and Barbuda','Argentina','Armenia','Australia','Austria','Azerbaijan','Bahamas','Bahrain','Bangladesh','Barbados','Belarus','Belgium','Belize','Benin','Bhutan','Bolivia','Bosnia and Herzegovina','Botswana','Brazil','Brunei','Bulgaria','Burkina Faso','Burundi','Cambodia','Cameroon','Canada','Cape Verde','Central African Republic','Chad','Chile','China','Colombia','Comoros','Congo','Costa Rica','Croatia','Cuba','Cyprus','Czech Republic','Democratic Republic of the Congo','Denmark','Djibouti','Dominica','Dominican Republic','East Timor','Ecuador','Egypt','El Salvador','Equatorial Guinea','Eritrea','Estonia','Eswatini','Ethiopia','Fiji','Finland','France','Gabon','Gambia','Georgia','Germany','Ghana','Greece','Grenada','Guatemala','Guinea','Guinea-Bissau','Guyana','Haiti','Honduras','Hungary','Iceland','India','Indonesia','Iran','Iraq','Ireland','Israel','Italy','Ivory Coast','Jamaica','Japan','Jordan','Kazakhstan','Kenya','Kiribati','Kosovo','Kuwait','Kyrgyzstan','Laos','Latvia','Lebanon','Lesotho','Liberia','Libya','Liechtenstein','Lithuania','Luxembourg','Madagascar','Malawi','Malaysia','Maldives','Mali','Malta','Marshall Islands','Mauritania','Mauritius','Mexico','Micronesia','Moldova','Monaco','Mongolia','Montenegro','Morocco','Mozambique','Myanmar','Namibia','Nauru','Nepal','Netherlands','New Zealand','Nicaragua','Niger','Nigeria','North Korea','North Macedonia','Norway','Oman','Pakistan','Palau','Palestine','Panama','Papua New Guinea','Paraguay','Peru','Philippines','Poland','Portugal','Puerto Rico','Qatar','Romania','Russia','Rwanda','Saint Kitts and Nevis','Saint Lucia','Saint Vincent and the Grenadines','Samoa','San Marino','Sao Tome and Principe','Saudi Arabia','Senegal','Serbia','Seychelles','Sierra Leone','Singapore','Slovakia','Slovenia','Solomon Islands','Somalia','South Africa','South Korea','South Sudan','Spain','Sri Lanka','Sudan','Suriname','Sweden','Switzerland','Syria','Taiwan','Tajikistan','Tanzania','Thailand','Togo','Tonga','Trinidad and Tobago','Tunisia','Turkey','Turkmenistan','Tuvalu','Uganda','Ukraine','United Arab Emirates','United Kingdom','United States','Uruguay','Uzbekistan','Vanuatu','Vatican City','Venezuela','Vietnam','Yemen','Zambia','Zimbabwe']
$f$;

CREATE OR REPLACE FUNCTION normalise_country(v text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  WITH s AS (SELECT lower(go_trim(coalesce(v, ''))) AS s)
  SELECT CASE WHEN s.s = '' THEN '' ELSE coalesce((SELECT c FROM unnest(countries()) c WHERE lower(c) = s.s LIMIT 1),
    '{"us":"United States","usa":"United States","u.s.":"United States","u.s.a.":"United States","america":"United States","uk":"United Kingdom","england":"United Kingdom","prc":"China","hong kong":"China","holland":"Netherlands","viet":"Vietnam","ca":"Canada","in":"India","cn":"China","mx":"Mexico","tr":"Turkey","pt":"Portugal","it":"Italy","vn":"Vietnam","id":"Indonesia","th":"Thailand","ph":"Philippines","kh":"Cambodia","korea":"South Korea","republic of korea":"South Korea","czechia":"Czech Republic","türkiye":"Turkey","turkiye":"Turkey","burma":"Myanmar","cote d''ivoire":"Ivory Coast","côte d''ivoire":"Ivory Coast","swaziland":"Eswatini","uae":"United Arab Emirates","great britain":"United Kingdom","viet nam":"Vietnam","macau":"China","macao":"China"}'::jsonb ->> s.s, '') END FROM s
$f$;

CREATE OR REPLACE FUNCTION measured_side(cell text, measure text) RETURNS double precision LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE p text; n double precision; sides double precision[] := '{}'; longest double precision; total double precision := 0;
BEGIN
  FOREACH p IN ARRAY regexp_split_to_array(go_trim(cell), '\s*[x×X]\s*') LOOP
    IF p = '' THEN CONTINUE; END IF;
    n := js_number(regexp_replace(p, '["in\s]', '', 'g'));
    IF n IS NULL THEN RETURN NULL; END IF;
    sides := sides || n;
  END LOOP;
  IF cardinality(sides) = 0 THEN RETURN NULL; END IF;
  longest := (SELECT max(x) FROM unnest(sides) x);
  total := (SELECT sum(x) FROM unnest(sides) x);
  RETURN CASE measure WHEN 'longest' THEN longest WHEN 'shortest' THEN (SELECT min(x) FROM unnest(sides) x) WHEN 'girth' THEN longest + 2 * (total - longest) END;
END $f$;

CREATE OR REPLACE FUNCTION test_passes(v jsonb, t jsonb) RETURNS boolean LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE s text := lower(go_trim(coalesce(v ->> (t ->> 'field'), ''))); op text := t ->> 'op'; first text := coalesce(t -> 'values' ->> 0, ''); line double precision; p text; n double precision; hit boolean;
BEGIN
  IF op = 'filled' THEN RETURN s <> ''; END IF;
  IF op IN ('above', 'below') THEN
    IF go_trim(first) !~ '^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$' THEN RETURN false; END IF;
    line := go_trim(first)::double precision;
    FOREACH p IN ARRAY multi_parts(s) LOOP
      IF coalesce(t ->> 'measure', '') <> '' THEN n := measured_side(p, t ->> 'measure');
      ELSE n := js_number(regexp_replace(p, '[^\d.-]', '', 'g'));
      END IF;
      IF n IS NOT NULL AND ((op = 'above' AND n > line) OR (op = 'below' AND n < line)) THEN RETURN true; END IF;
    END LOOP;
    RETURN false;
  END IF;
  IF op = 'contains' THEN RETURN strpos(s, lower(first)) > 0; END IF;
  hit := EXISTS (SELECT FROM jsonb_array_elements_text(coalesce(t -> 'values', '[]')) o, unnest(string_to_array(s, ';')) q WHERE go_trim(q) <> '' AND go_trim(q) = lower(o));
  RETURN CASE WHEN op = 'not' THEN NOT hit ELSE hit END;
END $f$;

CREATE OR REPLACE FUNCTION when_holds(v jsonb, w jsonb) RETURNS boolean LANGUAGE sql IMMUTABLE AS $f$
  SELECT NOT EXISTS (SELECT FROM jsonb_array_elements(CASE WHEN jsonb_typeof(w) = 'array' THEN w ELSE '[]' END) t WHERE NOT test_passes(v, t))
$f$;

CREATE OR REPLACE FUNCTION test_words(t jsonb, voice text) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE first text := coalesce(t -> 'values' ->> 0, ''); list text := (SELECT coalesce(string_agg(x, ' or '), '') FROM jsonb_array_elements_text(coalesce(t -> 'values', '[]')) x); w text[] := CASE voice WHEN 'condition' THEN ARRAY['is ', 'is not ', 'contains ', 'is above ', 'is below '] ELSE ARRAY['be ', 'not be ', 'contain ', 'be above ', 'be below '] END;
BEGIN
  RETURN CASE t ->> 'op' WHEN 'filled' THEN rtrim(w[1], ' ') || ' filled in' WHEN 'above' THEN w[4] || first WHEN 'below' THEN w[5] || first WHEN 'contains' THEN w[3] || first WHEN 'not' THEN w[2] || list ELSE w[1] || list END;
END $f$;

CREATE OR REPLACE FUNCTION when_words(labels jsonb, w jsonb) RETURNS text LANGUAGE sql IMMUTABLE AS $f$
  SELECT coalesce(string_agg(coalesce(labels ->> (t ->> 'field'), t ->> 'field')
    || CASE t ->> 'measure' WHEN 'longest' THEN '''s longest side' WHEN 'shortest' THEN '''s shortest side' WHEN 'girth' THEN '''s length plus girth' ELSE '' END
    || ' ' || test_words(t, 'condition'), ' and ' ORDER BY n), '') FROM jsonb_array_elements(w) WITH ORDINALITY AS q(t, n)
$f$;

CREATE OR REPLACE FUNCTION required_now(labels jsonb, f jsonb, v jsonb) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE w jsonb;
BEGIN
  IF coalesce((f ->> 'required')::boolean, false) THEN RETURN 'Required'; END IF;
  FOR w IN SELECT x FROM jsonb_array_elements(coalesce(f -> 'requiredWhen', '[]')) x LOOP
    IF jsonb_array_length(w) > 0 AND when_holds(v, w) THEN RETURN 'Required when ' || when_words(labels, w); END IF;
  END LOOP;
  RETURN '';
END $f$;

CREATE OR REPLACE FUNCTION check_now(labels jsonb, f jsonb, v jsonb) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE c jsonb; t jsonb; said text;
BEGIN
  FOR c IN SELECT x FROM jsonb_array_elements(coalesce(f -> 'must', '[]')) x LOOP
    t := jsonb_build_object('field', f ->> 'key', 'op', c ->> 'op', 'values', coalesce(c -> 'values', '[]'));
    IF when_holds(v, c -> 'when') AND NOT test_passes(v, t) THEN
      said := go_trim(coalesce(c ->> 'message', ''));
      IF said <> '' THEN RETURN said; END IF;
      said := 'Must ' || test_words(t, 'check');
      IF jsonb_array_length(coalesce(c -> 'when', '[]')) > 0 THEN said := said || ' when ' || when_words(labels, c -> 'when'); END IF;
      RETURN said;
    END IF;
  END LOOP;
  RETURN '';
END $f$;

CREATE OR REPLACE FUNCTION shape_issue(f jsonb, raw text) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE ty text := f ->> 'type'; multi boolean := coalesce((f ->> 'multi')::boolean, false); opts jsonb := coalesce(f -> 'options', '[]'); key text := f ->> 'key'; part text; i int := 0; one jsonb; bad text; txt text; n double precision; hit text; parts text[]; names text[]; want int; ok boolean; below boolean := false; ex text[] := '{}';
BEGIN
  IF multi AND ty <> 'select' AND ty NOT IN ('yesno', 'country') THEN
    FOREACH part IN ARRAY multi_parts(raw) LOOP
      i := i + 1;
      one := shape_issue(f || '{"multi": false}', part);
      IF one IS NOT NULL THEN
        RETURN jsonb_build_object('level', one ->> 'level', 'message', format('Answer %s, “%s”: %s. One per part, separated by ;', i, part, one ->> 'message'));
      END IF;
    END LOOP;
    RETURN NULL;
  END IF;
  IF ty = 'select' THEN
    parts := CASE WHEN multi THEN multi_parts(raw) ELSE ARRAY[raw] END;
    FOREACH part IN ARRAY parts LOOP
      IF NOT EXISTS (SELECT FROM jsonb_array_elements_text(opts) o WHERE lower(o) = lower(part)) THEN bad := part; EXIT; END IF;
    END LOOP;
    IF bad IS NULL THEN RETURN NULL; END IF;
    txt := (SELECT coalesce(string_agg(o, ', ' ORDER BY k), '') FROM jsonb_array_elements_text(opts) WITH ORDINALITY AS q(o, k));
    IF multi THEN RETURN jsonb_build_object('level', 'error', 'message', '“' || bad || '” is not one of: ' || txt); END IF;
    RETURN jsonb_build_object('level', 'error', 'message', 'Choose one of: ' || txt);
  ELSIF ty = 'yesno' THEN
    IF lower(raw) NOT IN ('yes', 'no') THEN RETURN jsonb_build_object('level', 'error', 'message', 'Yes or No'); END IF;
  ELSIF ty = 'number' THEN
    txt := number_text(raw);
    n := js_number(txt);
    IF n IS NULL THEN RETURN jsonb_build_object('level', 'error', 'message', 'A number, no units'); END IF;
    IF key IN ('manufacturer_cost', 'vendor_imap', 'vendor_msrp', 'retail', 'retail_map') THEN
      IF raw ~ '\.\d{3,}' THEN RETURN jsonb_build_object('level', 'error', 'message', 'Two decimals at most'); END IF;
      IF n <= 0 THEN RETURN jsonb_build_object('level', 'error', 'message', 'Must be above zero'); END IF;
    END IF;
    IF n < 0 THEN RETURN jsonb_build_object('level', 'error', 'message', 'Can’t be negative'); END IF;
  ELSIF ty = 'country' THEN
    hit := normalise_country(raw);
    IF hit = '' THEN RETURN jsonb_build_object('level', 'error', 'message', 'Full country name, e.g. India (not IN)'); END IF;
    IF NOT raw = ANY(countries()) THEN RETURN jsonb_build_object('level', 'warn', 'message', 'Did you mean ' || hit || '?'); END IF;
  ELSIF ty = 'dimensions' THEN
    parts := ARRAY(SELECT x FROM regexp_split_to_table(raw, '\s*[x×X]\s*') x WHERE x <> '');
    IF jsonb_typeof(f -> 'parts') = 'array' THEN names := ARRAY(SELECT jsonb_array_elements_text(f -> 'parts')); ELSE names := ARRAY['W', 'D', 'H']; END IF;
    want := cardinality(names);
    ok := cardinality(parts) = want;
    FOREACH part IN ARRAY parts LOOP
      n := js_number(regexp_replace(part, '["in\s]', '', 'g'));
      IF n IS NULL THEN ok := false; ELSIF n < 0 THEN below := true; END IF;
    END LOOP;
    IF ok AND below THEN RETURN jsonb_build_object('level', 'error', 'message', 'Can’t be negative'); END IF;
    IF NOT ok THEN
      FOR i IN 0 .. want - 1 LOOP ex := ex || (24 - i * 6)::text; END LOOP;
      RETURN jsonb_build_object('level', 'error', 'message', array_to_string(names, ' x ') || ' in inches, e.g. ' || array_to_string(ex, ' x '));
    END IF;
  END IF;
  RETURN NULL;
END $f$;

CREATE OR REPLACE FUNCTION pct_issue(raw text) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE p text; m text[]; total double precision := 0;
BEGIN
  FOREACH p IN ARRAY regexp_split_to_array(raw, '\s*,\s*|\s+[aA][nN][dD]\s+') LOOP
    IF p = '' THEN CONTINUE; END IF;
    m := regexp_match(p, '^(\d+(?:\.\d+)?)\s*%\s*\S');
    IF m IS NULL THEN RETURN 'Format: 70% cotton, 30% linen'; END IF;
    total := total + m[1]::double precision;
  END LOOP;
  IF abs(total - 100) > 1 THEN RETURN 'Percentages should add to 100% (got ' || total::text || '%)'; END IF;
  RETURN '';
END $f$;

CREATE OR REPLACE FUNCTION angles() RETURNS jsonb LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT '[{"key":"full","label":"Full","hint":"the whole product, straight on","studio":true},{"key":"three_quarter","label":"¾ angle","hint":"the whole product turned three-quarters","studio":true},{"key":"side","label":"Side","hint":"the whole product from the side","studio":true},{"key":"back","label":"Back","hint":"the whole product from behind, or its second side","studio":true},{"key":"open","label":"Open","hint":"the product with its doors or drawers open","studio":true},{"key":"corner","label":"Corner","hint":"a close view of one corner"},{"key":"detail","label":"Detail","hint":"a close-up of part of the product","many":true},{"key":"swatch","label":"Swatch","hint":"the material or finish filling the frame"},{"key":"set","label":"Set","hint":"every piece of the set together, or stacked"},{"key":"placesetting","label":"Place setting","hint":"one place set with the pieces"},{"key":"scale","label":"Scale","hint":"the product styled with a prop that shows its size"},{"key":"lifestyle","label":"Lifestyle","hint":"the product in a room or a styled scene","many":true},{"key":"repeat","label":"Repeat","hint":"the pattern''s digital repeat file, or the full panel"},{"key":"video","label":"Video","hint":"a GIF or video of the product moving","motion":true}]'::jsonb
$f$;

CREATE OR REPLACE FUNCTION angle_of(role text) RETURNS jsonb LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT a FROM jsonb_array_elements(angles()) a WHERE a ->> 'key' = role
$f$;

CREATE OR REPLACE FUNCTION images_owed(spec jsonb, v jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $f$
  WITH hit AS (
    SELECT r ->> 'role' AS role, max((r ->> 'min')::int) AS min, max(greatest((r ->> 'min')::int, coalesce((r ->> 'max')::int, 0))) AS max
    FROM jsonb_array_elements(CASE WHEN jsonb_typeof(spec -> 'images') = 'array' THEN spec -> 'images' ELSE '[]' END) r
    WHERE coalesce(r ->> 'role', '') <> '' AND coalesce((r ->> 'min')::int, 0) >= 1 AND when_holds(v, r -> 'when') GROUP BY 1)
  SELECT coalesce(jsonb_agg(jsonb_build_object('role', h.role, 'min', h.min, 'max', h.max) ORDER BY coalesce(o.n, 1000), h.role), '[]')
  FROM hit h LEFT JOIN (SELECT a ->> 'key' AS key, n FROM jsonb_array_elements(angles()) WITH ORDINALITY AS q(a, n)) o ON o.key = h.role
$f$;

CREATE OR REPLACE FUNCTION owed_message(role text, n int) RETURNS text LANGUAGE sql IMMUTABLE AS $f$
  SELECT CASE WHEN coalesce((a ->> 'motion')::boolean, false) THEN n || ' more GIF or video'
    ELSE n || ' more ' || coalesce(lower(a ->> 'label'), role) || CASE WHEN n > 1 THEN ' photos' ELSE ' photo' END END
  FROM (SELECT angle_of(role) AS a) q
$f$;

CREATE OR REPLACE FUNCTION format_word(media text) RETURNS text LANGUAGE sql IMMUTABLE AS $f$
  SELECT CASE WHEN media = '' THEN 'not a format we can read' WHEN media = 'jpeg' THEN 'a JPG' WHEN media IN ('heic', 'avif', 'webp', 'bmp', 'gif', 'png', 'tiff') THEN 'a ' || upper(media) ELSE 'a ' || media END
$f$;

CREATE OR REPLACE FUNCTION formats_words(p jsonb) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE w text[] := ARRAY(SELECT format_word(x) || CASE WHEN x = 'png' AND coalesce((p ->> 'transparentPng')::boolean, false) THEN ' on a transparent background' ELSE '' END FROM jsonb_array_elements_text(coalesce(p -> 'formats', '[]')) WITH ORDINALITY AS q(x, n) ORDER BY n); c int := cardinality(w);
BEGIN
  IF c = 0 THEN RETURN ''; END IF;
  IF c = 1 THEN RETURN w[1]; END IF;
  IF c = 2 THEN RETURN w[1] || ' or ' || w[2]; END IF;
  RETURN array_to_string(w[1:c - 1], ', ') || ', or ' || w[c];
END $f$;

CREATE OR REPLACE FUNCTION photo_issues(spec jsonb, row_id uuid, img build_image) RETURNS jsonb LANGUAGE plpgsql STABLE AS $f$
DECLARE out jsonb := '[]'; fid text := 'photo:' || img.id; a jsonb := angle_of(img.role); p jsonb := spec -> 'photo'; motion boolean := img.media IN ('gif', 'mp4', 'mov', 'webm'); video boolean := img.media IN ('mp4', 'mov', 'webm');
BEGIN
  IF NOT img.measured THEN RETURN out; END IF;
  IF coalesce((a ->> 'motion')::boolean, false) THEN
    IF NOT motion THEN RETURN jsonb_build_array(jsonb_build_object('row', row_id, 'field', fid, 'level', 'error', 'message', img.name || ' is a still picture; this slot takes a GIF or a video.')); END IF;
    RETURN out;
  END IF;
  IF video THEN RETURN jsonb_build_array(jsonb_build_object('row', row_id, 'field', fid, 'level', 'error', 'message', img.name || ' is a video; it goes in the Video slot.')); END IF;
  IF p IS NULL OR jsonb_typeof(p) <> 'object' THEN RETURN out; END IF;
  IF jsonb_array_length(coalesce(p -> 'formats', '[]')) > 0 AND NOT (p -> 'formats') ? img.media THEN
    RETURN jsonb_build_array(jsonb_build_object('row', row_id, 'field', fid, 'level', 'error', 'message', img.name || ' is ' || format_word(img.media) || '; send ' || formats_words(p) || '.'));
  END IF;
  IF img.media = 'png' AND coalesce((p ->> 'transparentPng')::boolean, false) AND NOT img.transparent THEN
    out := out || jsonb_build_object('row', row_id, 'field', fid, 'level', 'error', 'message', img.name || ' is a PNG without a transparent background; send it as a JPG or TIFF, or cut out on transparent.');
  END IF;
  IF img.width > 0 AND (img.width < coalesce((p ->> 'minWidth')::int, 0) OR img.height < coalesce((p ->> 'minHeight')::int, 0)) THEN
    out := out || jsonb_build_object('row', row_id, 'field', fid, 'level', 'error', 'message', format('%s is %s×%s px; it needs to be at least %s×%s.', img.name, img.width, img.height, coalesce((p ->> 'minWidth')::int, 0), coalesce((p ->> 'minHeight')::int, 0)));
  END IF;
  IF a IS NULL OR NOT coalesce((a ->> 'studio')::boolean, false) THEN RETURN out; END IF;
  IF coalesce((p ->> 'fill')::real, 0) > 0 AND img.fill > 0 AND img.fill < (p ->> 'fill')::real THEN
    out := out || jsonb_build_object('row', row_id, 'field', fid, 'level', 'warn', 'message', 'The product looks small in ' || img.name || '; it should fill the frame.');
  END IF;
  RETURN out;
END $f$;

CREATE OR REPLACE FUNCTION go_quote(s text) RETURNS text LANGUAGE sql IMMUTABLE AS $f$
  SELECT '"' || replace(replace(replace(replace(replace(s, '\', '\\'), '"', '\"'), E'\n', '\n'), E'\t', '\t'), E'\r', '\r') || '"'
$f$;

CREATE OR REPLACE FUNCTION narrow_when(w jsonb, by_key jsonb) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE out jsonb := '[]'; t jsonb; f jsonb; choices jsonb; kept jsonb;
BEGIN
  IF w IS NULL OR jsonb_typeof(w) <> 'array' OR jsonb_array_length(w) = 0 THEN RETURN '[]'; END IF;
  FOR t IN SELECT x FROM jsonb_array_elements(w) x LOOP
    f := by_key -> (t ->> 'field');
    IF f IS NULL THEN RETURN NULL; END IF;
    choices := CASE f ->> 'type' WHEN 'select' THEN f -> 'options' WHEN 'yesno' THEN '["Yes", "No"]' WHEN 'country' THEN to_jsonb(countries()) END;
    IF choices IS NOT NULL AND jsonb_typeof(choices) = 'array' AND coalesce(t ->> 'op', 'is') = 'is' THEN
      kept := (SELECT coalesce(jsonb_agg(x ORDER BY n), '[]') FROM jsonb_array_elements_text(coalesce(t -> 'values', '[]')) WITH ORDINALITY AS q(x, n) WHERE choices ? x);
      IF jsonb_array_length(kept) = 0 THEN RETURN NULL; END IF;
      t := t || jsonb_build_object('values', kept);
    END IF;
    out := out || jsonb_build_array(t);
  END LOOP;
  RETURN out;
END $f$;

CREATE OR REPLACE FUNCTION field_json(f field) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $f$
  SELECT jsonb_strip_nulls(jsonb_build_object('key', f.key, 'label', go_trim(f.name), 'type', f.type, 'section', f.section, 'owner', f.owner, 'required', f.required, 'requiredWhen', f.required_when, 'must', f.must, 'fills', f.fills,
    'options', CASE WHEN f.options IS NOT NULL THEN (SELECT coalesce(jsonb_agg(go_trim(o) ORDER BY n), '[]') FROM unnest(f.options) WITH ORDINALITY AS q(o, n)) END,
    'parts', to_jsonb(f.parts), 'help', nullif(f.help, ''), 'note', nullif(f.note, ''), 'multi', CASE WHEN f.multi THEN true END, 'pctBreakdown', CASE WHEN f.pct_breakdown THEN true END))
$f$;

CREATE OR REPLACE FUNCTION template_spec(t uuid) RETURNS jsonb LANGUAGE plpgsql STABLE AS $f$
DECLARE
  chain template[] := '{}'; r template; cur uuid := t; ids uuid[] := '{}'; keys text[] := '{}'; vals jsonb[] := '{}'; seen text[] := '{}'; f field; i int; images jsonb; photo jsonb; ex_row jsonb; ex_cols jsonb := '{}'; ex_comp jsonb := '{}';
  c record; ls text[]; l text; meta jsonb; names text[]; chain_keys text[]; reading text[]; by_key jsonb := '{}'; out jsonb := '[]'; x jsonb; alts jsonb; n jsonb; leaf template; rule jsonb; imgs jsonb;
BEGIN
  WHILE cur IS NOT NULL AND NOT cur = ANY(ids) LOOP
    SELECT * INTO r FROM template WHERE id = cur;
    EXIT WHEN NOT FOUND;
    ids := ids || cur;
    chain := r || chain;
    cur := r.parent;
  END LOOP;
  IF cardinality(chain) = 0 THEN RETURN NULL; END IF;
  leaf := chain[cardinality(chain)];
  chain_keys := ARRAY(SELECT x_t.key FROM unnest(chain) AS x_t);
  FOREACH r IN ARRAY chain LOOP
    FOR f IN SELECT * FROM field WHERE template = r.id ORDER BY position, id LOOP
      seen := seen || f.key;
      i := array_position(keys, f.key);
      IF i IS NULL THEN keys := keys || f.key; vals := vals || field_json(f); ELSE vals[i] := field_json(f); END IF;
    END LOOP;
    IF cardinality(r.hides) > 0 THEN
      vals := ARRAY(SELECT v FROM unnest(keys, vals) AS u(k, v) WHERE NOT k = ANY(r.hides));
      keys := ARRAY(SELECT k FROM unnest(keys) k WHERE NOT k = ANY(r.hides));
    END IF;
    IF r.images IS NOT NULL AND jsonb_typeof(r.images) = 'array' THEN images := r.images; END IF;
    IF r.photo IS NOT NULL AND jsonb_typeof(r.photo) = 'object' THEN photo := r.photo; END IF;
    IF coalesce((r.export ->> 'row')::int, 0) > 0 THEN ex_row := r.export -> 'row'; END IF;
    ex_cols := ex_cols || coalesce(r.export -> 'columns', '{}');
    ex_comp := ex_comp || coalesce(r.export -> 'compose', '{}');
  END LOOP;
  FOR c IN SELECT a.attname::text AS key, d.description FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_description d ON d.objoid = a.attrelid AND d.objsubid = a.attnum
    WHERE a.attrelid = 'public.build_row'::regclass AND NOT a.attisdropped AND d.description ~ '(^|\n)templates:' AND NOT a.attname::text = ANY(seen) ORDER BY a.attnum LOOP
    ls := string_to_array(c.description, E'\n');
    meta := jsonb_build_object('key', c.key, 'label', go_trim(ls[1]), 'type', 'text', 'section', '', 'owner', 'vendor', 'required', false);
    names := '{}';
    FOREACH l IN ARRAY ls[2:] LOOP
      IF l ~ '^section:' THEN meta := meta || jsonb_build_object('section', go_trim(substr(l, 9)));
      ELSIF l ~ '^type:' THEN meta := meta || jsonb_build_object('type', go_trim(substr(l, 6)));
      ELSIF l ~ '^owner:' THEN meta := meta || jsonb_build_object('owner', go_trim(substr(l, 7)));
      ELSIF l ~ '^required:' THEN meta := meta || jsonb_build_object('required', go_trim(substr(l, 10)) = 'true');
      ELSIF l ~ '^options:' THEN meta := meta || jsonb_build_object('options', (SELECT jsonb_agg(go_trim(o)) FROM unnest(string_to_array(substr(l, 9), '|')) o));
      ELSIF l ~ '^templates:' THEN names := ARRAY(SELECT go_trim(o) FROM unnest(string_to_array(substr(l, 11), ',')) o);
      END IF;
    END LOOP;
    IF 'all' = ANY(names) OR names && chain_keys THEN keys := keys || c.key; vals := vals || meta; END IF;
  END LOOP;
  SELECT coalesce(jsonb_object_agg(k, v), '{}') INTO by_key FROM unnest(keys, vals) AS u(k, v);
  FOREACH x IN ARRAY vals LOOP
    IF x ? 'requiredWhen' THEN
      alts := '[]';
      FOR rule IN SELECT y FROM jsonb_array_elements(x -> 'requiredWhen') y LOOP
        n := narrow_when(rule, by_key);
        IF n IS NOT NULL AND jsonb_array_length(n) > 0 THEN alts := alts || jsonb_build_array(n); END IF;
      END LOOP;
      x := x || jsonb_build_object('requiredWhen', alts);
    END IF;
    IF x ? 'must' THEN
      alts := '[]';
      FOR rule IN SELECT y FROM jsonb_array_elements(x -> 'must') y LOOP
        n := narrow_when(rule -> 'when', by_key);
        IF n IS NOT NULL THEN alts := alts || jsonb_build_array(rule || jsonb_build_object('when', n)); END IF;
      END LOOP;
      x := x || jsonb_build_object('must', alts);
    END IF;
    IF x ? 'fills' THEN
      alts := '[]';
      FOR rule IN SELECT y FROM jsonb_array_elements(x -> 'fills') y LOOP
        n := narrow_when(rule -> 'when', by_key);
        IF n IS NOT NULL THEN alts := alts || jsonb_build_array(rule || jsonb_build_object('when', n)); END IF;
      END LOOP;
      x := x || jsonb_build_object('fills', alts);
    END IF;
    out := out || jsonb_build_array(x);
  END LOOP;
  IF images IS NOT NULL THEN
    imgs := '[]';
    FOR rule IN SELECT y FROM jsonb_array_elements(images) y LOOP
      n := narrow_when(rule -> 'when', by_key);
      IF n IS NOT NULL THEN imgs := imgs || jsonb_build_array(rule || jsonb_build_object('when', n)); END IF;
    END LOOP;
  END IF;
  SELECT i2.sections INTO reading FROM intake i2 LIMIT 1;
  reading := coalesce(reading, '{}');
  out := (SELECT coalesce(jsonb_agg(e ORDER BY coalesce(array_position(reading, e ->> 'section'), greatest(cardinality(reading), 1)), n2), '[]') FROM jsonb_array_elements(out) WITH ORDINALITY AS q(e, n2));
  ex_cols := (SELECT coalesce(jsonb_object_agg(k, v), '{}') FROM jsonb_each(ex_cols) AS e(k, v) WHERE by_key ? k);
  ex_comp := (SELECT coalesce(jsonb_object_agg(k, v), '{}') FROM jsonb_each(ex_comp) AS e(k, v) WHERE by_key ? k);
  RETURN jsonb_build_object('key', leaf.key, 'name', leaf.name, 'icon', leaf.icon, 'sheet', leaf.sheet, 'shell', leaf.shell, 'fields', out, 'images', imgs, 'photo', photo, 'export', jsonb_strip_nulls(jsonb_build_object('row', ex_row, 'columns', ex_cols, 'compose', ex_comp)));
END $f$;

CREATE OR REPLACE FUNCTION spec_of(t uuid) RETURNS jsonb LANGUAGE plpgsql AS $f$
DECLARE k text := t::text || '|' || statement_timestamp()::text; s jsonb;
BEGIN
  IF current_setting('intake.spec_key', true) = k THEN RETURN current_setting('intake.spec')::jsonb; END IF;
  s := template_spec(t);
  PERFORM set_config('intake.spec', s::text, true), set_config('intake.spec_key', k, true);
  RETURN s;
END $f$;

CREATE OR REPLACE FUNCTION set_default(v jsonb, d jsonb, key text, want text) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE have text := go_trim(coalesce(v ->> key, '')); mark text := d ->> key; marked boolean := d ? key;
BEGIN
  IF marked AND have <> mark THEN d := d - key; marked := false; END IF;
  IF have <> '' AND NOT marked THEN RETURN jsonb_build_array(v, d); END IF;
  IF want <> '' THEN v := v || jsonb_build_object(key, want); d := d || jsonb_build_object(key, want);
  ELSIF marked THEN v := v - key; d := d - key;
  END IF;
  RETURN jsonb_build_array(v, d);
END $f$;

CREATE OR REPLACE FUNCTION lead_text(mn int, mx int, unit text) RETURNS text LANGUAGE sql IMMUTABLE AS $f$
  SELECT CASE WHEN mn = mx THEN mx || ' ' ELSE mn || '-' || mx || ' ' END || CASE unit WHEN 'business_days' THEN 'business day' WHEN 'days' THEN 'day' WHEN 'weeks' THEN 'week' ELSE unit END || CASE WHEN mx <> 1 THEN 's' ELSE '' END
$f$;

CREATE OR REPLACE FUNCTION row_compute(v jsonb, d jsonb, row_id uuid, b build, spec_in jsonb, given_sku text) RETURNS jsonb LANGUAGE plpgsql AS $f$
DECLARE
  spec jsonb := spec_in; fields jsonb := coalesce(spec_in -> 'fields', '[]'); f jsonb; pair jsonb; key text; raw text; why text; ship text := ''; rules jsonb; want text; m text; labels jsonb; issues jsonb := '[]'; one jsonb; had int; blank text[]; sku text := given_sku; name text; weight_raw text; weight double precision; ask jsonb; img build_image; have int; errs int; warns int; keyed boolean; vfields jsonb; x jsonb; msg text;
BEGIN
  IF fields @? '$[*] ? (@.key == "build_type")' THEN v := v || jsonb_build_object('build_type', b.build_type); END IF;
  FOR f IN SELECT e FROM jsonb_array_elements(fields) e WHERE jsonb_array_length(coalesce(e -> 'fills', '[]')) > 0 LOOP
    want := '';
    FOR x IN SELECT y FROM jsonb_array_elements(f -> 'fills') y LOOP
      IF when_holds(v, x -> 'when') THEN want := x ->> 'value'; EXIT; END IF;
    END LOOP;
    pair := set_default(v, d, f ->> 'key', want);
    v := pair -> 0; d := pair -> 1;
  END LOOP;
  SELECT e ->> 'key' INTO ship FROM jsonb_array_elements(fields) WITH ORDINALITY AS q(e, n) WHERE e ->> 'type' = 'select' AND e ->> 'owner' = 'lg' AND (e ->> 'key' ~* '(^|[^a-z])ship(ping)?[ \t\n_-]*class($|[^a-z])' OR e ->> 'label' ~* '(^|[^a-z])ship(ping)?[ \t\n_-]*class($|[^a-z])') ORDER BY n LIMIT 1;
  IF ship IS NULL THEN
    SELECT e ->> 'key' INTO ship FROM jsonb_array_elements(fields) WITH ORDINALITY AS q(e, n) WHERE e ->> 'type' = 'select' AND (e ->> 'key' ~* '(^|[^a-z])ship(ping)?[ \t\n_-]*(method|via)($|[^a-z])' OR e ->> 'label' ~* '(^|[^a-z])ship(ping)?[ \t\n_-]*(method|via)($|[^a-z])') ORDER BY n LIMIT 1;
  END IF;
  FOR f IN SELECT e FROM jsonb_array_elements(fields) e WHERE e ->> 'type' NOT IN ('select', 'yesno') AND (e ->> 'key' ~* '(^|[^a-z])lead[ \t\n_-]*times?($|[^a-z])' OR e ->> 'label' ~* '(^|[^a-z])lead[ \t\n_-]*times?($|[^a-z])') LOOP
    IF rules IS NULL THEN
      rules := (SELECT coalesce(jsonb_agg(jsonb_build_object('m', lower(go_trim(coalesce(r.ship_method, ''))), 't', lead_text(r.min_time, r.max_time, r.unit::text))), '[]') FROM lead_time_rule r WHERE r.vendor = b.vendor);
    END IF;
    m := lower(go_trim(coalesce(v ->> ship, '')));
    want := coalesce(CASE WHEN m <> '' THEN (SELECT y ->> 't' FROM jsonb_array_elements(rules) y WHERE y ->> 'm' = m LIMIT 1) END, (SELECT y ->> 't' FROM jsonb_array_elements(rules) y WHERE y ->> 'm' = '' LIMIT 1), '');
    pair := set_default(v, d, f ->> 'key', want);
    v := pair -> 0; d := pair -> 1;
  END LOOP;
  keyed := fields @? '$[*] ? (@.key == "manufacturer_sku")';
  IF keyed THEN sku := go_trim(coalesce(v ->> 'manufacturer_sku', '')); END IF;
  name := go_trim(coalesce(v ->> 'vendor_product_name', ''));
  IF sku = '' AND name = '' THEN RAISE EXCEPTION 'A product needs its SKU or its name.' USING ERRCODE = 'P0001'; END IF;
  SELECT coalesce(jsonb_object_agg(e ->> 'key', e ->> 'label'), '{}') INTO labels FROM jsonb_array_elements(fields) e;
  blank := ARRAY(SELECT c.field || E'\x01' || lower(btrim(c.raw)) FROM cell_answer c WHERE NOT c.answers);
  FOR f IN SELECT e FROM jsonb_array_elements(fields) e WHERE e ->> 'owner' = 'vendor' LOOP
    key := f ->> 'key';
    raw := go_trim(coalesce(v ->> key, ''));
    IF raw <> '' AND cardinality(blank) > 0 AND f ->> 'type' = 'text' AND octet_length(raw) <= 60 AND key NOT IN ('manufacturer_sku', 'vendor_product_name') AND (key || E'\x01' || lower(raw)) = ANY(blank) THEN
      why := required_now(labels, f, v);
      IF why <> '' THEN
        issues := issues || jsonb_build_object('row', row_id, 'field', key, 'level', 'error', 'message', go_quote(raw) || ' is not an answer. ' || why);
        CONTINUE;
      END IF;
    END IF;
    IF raw = '' THEN
      why := required_now(labels, f, v);
      IF why = '' THEN why := check_now(labels, f, v); END IF;
      IF why <> '' THEN issues := issues || jsonb_build_object('row', row_id, 'field', key, 'level', 'error', 'message', why); END IF;
      CONTINUE;
    END IF;
    IF coalesce((f ->> 'pctBreakdown')::boolean, false) THEN
      msg := pct_issue(raw);
      one := CASE WHEN msg <> '' THEN jsonb_build_object('level', 'error', 'message', msg) END;
    ELSE
      one := shape_issue(f, raw);
    END IF;
    IF one IS NOT NULL THEN issues := issues || jsonb_build_object('row', row_id, 'field', key, 'level', one ->> 'level', 'message', one ->> 'message'); END IF;
    IF one IS NULL OR one ->> 'level' <> 'error' THEN
      why := check_now(labels, f, v);
      IF why <> '' THEN issues := issues || jsonb_build_object('row', row_id, 'field', key, 'level', 'error', 'message', why); END IF;
    END IF;
  END LOOP;
  weight_raw := CASE WHEN v ? 'shipping_weight_a' THEN v ->> 'shipping_weight_a' ELSE coalesce(v ->> 'shipping_weight', '') END;
  weight := js_number(regexp_replace(weight_raw, '[^\d.]', '', 'g'));
  IF weight > 150 AND lower(coalesce(v ->> 'shipping_method', '')) = 'ground' THEN
    issues := issues || jsonb_build_object('row', row_id, 'field', 'shipping_method', 'level', 'warn', 'message', 'Over 150 lb usually ships White Glove');
  END IF;
  IF lower(go_trim(coalesce(v ->> 'imported', ''))) = 'yes' AND normalise_country(v ->> 'country_of_origin') = 'United States' THEN
    issues := issues || jsonb_build_object('row', row_id, 'field', 'imported', 'level', 'warn', 'message', 'Imported, but country of origin is United States');
  END IF;
  IF b.photos THEN
    FOR ask IN SELECT y FROM jsonb_array_elements(images_owed(spec, v)) y LOOP
      SELECT count(*) INTO have FROM build_image i WHERE i.build_row = row_id AND i.role = ask ->> 'role';
      IF have < (ask ->> 'min')::int THEN
        issues := issues || jsonb_build_object('row', row_id, 'field', 'image:' || (ask ->> 'role'), 'level', 'error', 'message', owed_message(ask ->> 'role', (ask ->> 'min')::int - have));
      END IF;
    END LOOP;
    FOR img IN SELECT * FROM build_image i WHERE i.build_row = row_id AND i.role <> '' ORDER BY i.position, i.created_at LOOP
      issues := issues || photo_issues(spec, row_id, img);
    END LOOP;
  END IF;
  SELECT count(*) FILTER (WHERE e ->> 'level' = 'error'), count(*) FILTER (WHERE e ->> 'level' <> 'error') INTO errs, warns FROM jsonb_array_elements(issues) e;
  RETURN jsonb_build_object('values', v, 'defaults', d, 'issues', issues, 'errors', errs, 'warns', warns, 'status', CASE WHEN errs > 0 THEN 'fix' WHEN warns > 0 THEN 'note' ELSE 'ok' END, 'sku', sku, 'name', name);
END $f$;

CREATE OR REPLACE FUNCTION row_meta() RETURNS text[] LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT ARRAY['id', 'name', 'access', 'created_at', 'updated_at', 'search', 'build', 'sku', 'position', 'status', 'seeded', 'lg_seeded', 'defaults', 'lg_defaults', 'listed', 'issues', 'errors', 'warns', 'copied']
$f$;

CREATE OR REPLACE FUNCTION row_values(r build_row) RETURNS jsonb LANGUAGE sql STABLE AS $f$
  SELECT jsonb_strip_nulls(to_jsonb(r) - row_meta())
$f$;

CREATE OR REPLACE FUNCTION row_recheck(r build_row) RETURNS jsonb LANGUAGE plpgsql AS $f$
DECLARE b build;
BEGIN
  SELECT * INTO b FROM build WHERE id = r.build;
  RETURN row_compute(row_values(r), r.defaults || r.lg_defaults, r.id, b, spec_of(b.template), r.sku);
END $f$;

CREATE OR REPLACE FUNCTION spelled(f jsonb, raw text) RETURNS text LANGUAGE plpgsql IMMUTABLE AS $f$
DECLARE ty text := f ->> 'type'; multi boolean := coalesce((f ->> 'multi')::boolean, false); parts text[]; said text[] := '{}'; p text; o text; txt text; v text := raw;
BEGIN
  IF f IS NULL THEN RETURN raw; END IF;
  IF ty = 'yesno' THEN
    IF lower(go_trim(v)) IN ('y', 'yes', 'true', '1', 'x', '✓') THEN RETURN 'Yes'; END IF;
    IF lower(go_trim(v)) IN ('n', 'no', 'false', '0') THEN RETURN 'No'; END IF;
    RETURN v;
  END IF;
  IF ty = 'select' THEN
    parts := CASE WHEN multi THEN multi_parts(v) ELSE ARRAY[v] END;
    IF cardinality(parts) = 0 THEN RETURN v; END IF;
    FOREACH p IN ARRAY parts LOOP
      o := (SELECT x FROM jsonb_array_elements_text(coalesce(f -> 'options', '[]')) x WHERE squash(x) = squash(p) AND squash(p) <> '' LIMIT 1);
      IF o IS NULL THEN RETURN v; END IF;
      IF NOT o = ANY(said) THEN said := said || o; END IF;
    END LOOP;
    RETURN array_to_string(said, '; ');
  END IF;
  IF ty = 'number' THEN
    IF multi THEN
      parts := multi_parts(v);
      FOREACH p IN ARRAY parts LOOP
        txt := number_text(p);
        IF js_number(txt) IS NULL THEN RETURN v; END IF;
        said := said || txt;
      END LOOP;
      RETURN array_to_string(said, '; ');
    END IF;
    txt := number_text(v);
    IF js_number(txt) IS NOT NULL THEN RETURN txt; END IF;
    RETURN v;
  END IF;
  IF ty IN ('dimensions', 'text') AND multi AND NOT coalesce((f ->> 'pctBreakdown')::boolean, false) THEN
    parts := multi_parts(v);
    IF cardinality(parts) > 0 THEN RETURN array_to_string(parts, '; '); END IF;
  END IF;
  RETURN v;
END $f$;

CREATE OR REPLACE FUNCTION squash(s text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT regexp_replace(lower(coalesce(s, '')), '[^a-z0-9]', '', 'g')
$f$;

CREATE OR REPLACE FUNCTION stage_words(stage text) RETURNS text LANGUAGE sql IMMUTABLE AS $f$
  SELECT CASE stage WHEN 'draft' THEN 'a draft' WHEN 'sent' THEN 'sent and not yet opened' WHEN 'in_progress' THEN 'in progress with the vendor' WHEN 'review' THEN 'in review' WHEN 'complete' THEN 'approved' WHEN 'cancelled' THEN 'cancelled' ELSE stage END
$f$;

CREATE OR REPLACE FUNCTION build_log(bid uuid, who text, line text, at timestamptz DEFAULT now()) RETURNS void @DEFINER@ AS $f$
BEGIN
  INSERT INTO build_activity (build, at, who, text, name) VALUES (bid, at, who, line, line);
END $f$;

CREATE OR REPLACE FUNCTION build_recount(bid uuid) RETURNS void @DEFINER@ AS $f$
DECLARE b build; n int; ok int; errs int; warned int; brand text;
BEGIN
  SELECT * INTO b FROM build WHERE id = bid FOR UPDATE;
  IF NOT FOUND THEN RETURN; END IF;
  SELECT count(*), count(*) FILTER (WHERE r.errors = 0), coalesce(sum(r.errors), 0), coalesce(sum(r.warns), 0) INTO n, ok, errs, warned FROM build_row r WHERE r.build = bid;
  UPDATE build SET skus = n, clean = ok, errors = errs, warns = warned WHERE id = bid;
  IF b.stage IN ('complete', 'cancelled', 'draft') THEN RETURN; END IF;
  IF b.stage = 'sent' THEN
    UPDATE build SET stage = 'in_progress' WHERE id = bid;
    b.stage := 'in_progress';
  END IF;
  IF b.stage = 'in_progress' AND errs = 0 AND n > 0 AND b.returned_at IS NULL THEN
    UPDATE build SET stage = 'review', submitted_at = now(), return_note = '' WHERE id = bid;
    brand := (SELECT i.brand FROM intake i LIMIT 1);
    PERFORM build_log(bid, 'vendor', format('Complete: %s of %s SKUs — sent to %s', n, n, brand));
  ELSIF b.stage = 'review' AND errs > 0 THEN
    UPDATE build SET stage = 'in_progress', submitted_at = NULL WHERE id = bid;
    PERFORM build_log(bid, 'system', format('Back with the vendor: %s error%s open again', errs, CASE WHEN errs = 1 THEN '' ELSE 's' END));
  END IF;
END $f$;

CREATE OR REPLACE FUNCTION build_row_stamp() RETURNS trigger @DEFINER@ AS $f$
DECLARE
  b build; spec jsonb; v jsonb := row_values(NEW); old_v jsonb := CASE WHEN TG_OP = 'UPDATE' THEN row_values(OLD) ELSE '{}' END; scoped boolean := archive.scope() IS NOT NULL; k text; f jsonb; val text; by_key jsonb; out jsonb; d jsonb; lg text[]; patch jsonb; the_sku text; the_name text;
BEGIN
  SELECT * INTO b FROM build WHERE id = NEW.build;
  IF NOT FOUND THEN RAISE EXCEPTION 'no such build' USING ERRCODE = 'P0001'; END IF;
  IF b.stage IN ('complete', 'cancelled') THEN RAISE EXCEPTION 'This build is % and takes no more changes.', stage_words(b.stage) USING ERRCODE = 'P0001'; END IF;
  spec := spec_of(b.template);
  SELECT coalesce(jsonb_object_agg(e ->> 'key', e), '{}') INTO by_key FROM jsonb_array_elements(coalesce(spec -> 'fields', '[]')) e;
  lg := ARRAY(SELECT e ->> 'key' FROM jsonb_array_elements(coalesce(spec -> 'fields', '[]')) e WHERE e ->> 'owner' = 'lg');
  IF TG_OP = 'UPDATE' THEN
    FOR k IN SELECT key FROM jsonb_each_text(v) WHERE old_v ->> key IS DISTINCT FROM value UNION SELECT key FROM jsonb_each_text(old_v) WHERE NOT v ? key LOOP
      f := by_key -> k;
      IF f IS NULL THEN RAISE EXCEPTION '% is not a field of this template.', k USING ERRCODE = 'P0001'; END IF;
      IF scoped AND k IN ('manufacturer_sku', 'vendor_product_name') AND k = ANY(OLD.listed) THEN
        RAISE EXCEPTION 'The % was listed by the buyer and cannot be changed.', CASE k WHEN 'manufacturer_sku' THEN 'manufacturer SKU' ELSE 'product name' END USING ERRCODE = 'P0001';
      END IF;
      IF k = 'build_type' AND v ? k AND b.build_type IS DISTINCT FROM v ->> k THEN
        RAISE EXCEPTION 'Build type is the build''s own, picked when it was made; every product takes it from there.' USING ERRCODE = 'P0001';
      END IF;
      val := go_trim(spelled(f, go_trim(coalesce(v ->> k, ''))));
      IF val = '' THEN v := v - k; ELSE v := v || jsonb_build_object(k, val); END IF;
    END LOOP;
  ELSE
    FOR k IN SELECT key FROM jsonb_each_text(v) LOOP
      IF NOT by_key ? k THEN v := v - k; END IF;
    END LOOP;
    NEW.seeded := (SELECT coalesce(jsonb_object_agg(key, value), '{}') FROM jsonb_each(v) WHERE NOT key = ANY(lg));
    NEW.lg_seeded := (SELECT coalesce(jsonb_object_agg(key, value), '{}') FROM jsonb_each(v) WHERE key = ANY(lg));
    NEW.listed := ARRAY(SELECT x FROM unnest(ARRAY['manufacturer_sku', 'vendor_product_name']) x WHERE go_trim(coalesce(v ->> x, '')) <> '');
  END IF;
  out := row_compute(v, NEW.defaults || NEW.lg_defaults, NEW.id, b, spec, NEW.sku);
  v := out -> 'values';
  d := out -> 'defaults';
  the_sku := out ->> 'sku';
  the_name := out ->> 'name';
  IF TG_OP = 'INSERT' OR the_sku IS DISTINCT FROM OLD.sku OR the_name IS DISTINCT FROM OLD.name THEN
    IF the_sku <> '' AND EXISTS (SELECT FROM build_row o WHERE o.build = NEW.build AND o.id <> NEW.id AND lower(btrim(o.sku)) = lower(btrim(the_sku))) THEN
      RAISE EXCEPTION '%', CASE WHEN TG_OP = 'INSERT' THEN 'Another product on this build already has that SKU.' ELSE format('Another product on this build already has the SKU %s.', the_sku) END USING ERRCODE = 'P0001';
    END IF;
    IF the_sku = '' AND the_name <> '' AND EXISTS (SELECT FROM build_row o WHERE o.build = NEW.build AND o.id <> NEW.id AND btrim(o.sku) = '' AND lower(btrim(o.name)) = lower(btrim(the_name))) THEN
      RAISE EXCEPTION '%', CASE WHEN TG_OP = 'INSERT' THEN 'Another product on this build already has that name.' ELSE format('Another product on this build is already called %s.', the_name) END USING ERRCODE = 'P0001';
    END IF;
  END IF;
  SELECT coalesce(jsonb_object_agg(key, CASE WHEN v ? key THEN v -> key ELSE 'null'::jsonb END), '{}') INTO patch FROM (SELECT jsonb_object_keys(v) AS key UNION SELECT jsonb_object_keys(row_values(NEW))) q;
  NEW := jsonb_populate_record(NEW, patch);
  NEW.defaults := (SELECT coalesce(jsonb_object_agg(key, value), '{}') FROM jsonb_each(d) WHERE NOT key = ANY(lg));
  NEW.lg_defaults := (SELECT coalesce(jsonb_object_agg(key, value), '{}') FROM jsonb_each(d) WHERE key = ANY(lg));
  NEW.issues := out -> 'issues';
  NEW.errors := (out ->> 'errors')::int;
  NEW.warns := (out ->> 'warns')::int;
  NEW.status := out ->> 'status';
  NEW.sku := the_sku;
  NEW.name := the_name;
  RETURN NEW;
END $f$;

CREATE OR REPLACE FUNCTION build_row_counted() RETURNS trigger @DEFINER@ AS $f$
DECLARE bid uuid;
BEGIN
  FOR bid IN SELECT DISTINCT build FROM changed LOOP
    PERFORM build_recount(bid);
  END LOOP;
  RETURN NULL;
END $f$;

CREATE OR REPLACE FUNCTION build_rows_again(bid uuid) RETURNS void @DEFINER@ AS $f$
BEGIN
  UPDATE build_row SET updated_at = now() WHERE build = bid;
END $f$;

CREATE OR REPLACE FUNCTION build_rows_reset(bid uuid) RETURNS void @DEFINER@ AS $f$
DECLARE cols text;
BEGIN
  SELECT string_agg(format('%I = (r.seeded || r.lg_seeded) ->> %L', a.attname, a.attname), ', ') INTO cols
    FROM pg_catalog.pg_attribute a WHERE a.attrelid = 'public.build_row'::regclass AND a.attnum > 0 AND NOT a.attisdropped AND a.atttypid = 'text'::regtype AND NOT a.attname::text = ANY(row_meta());
  IF cols IS NULL THEN RETURN; END IF;
  EXECUTE format('UPDATE build_row r SET defaults = ''{}'', lg_defaults = ''{}'', updated_at = now(), %s WHERE r.build = $1', cols) USING bid;
END $f$;

CREATE OR REPLACE FUNCTION build_rows_seed(bid uuid, rows jsonb) RETURNS void @DEFINER@ AS $f$
DECLARE cols text; base jsonb := jsonb_build_object('build', bid, 'created_at', now(), 'updated_at', now(), 'seeded', '{}'::jsonb, 'lg_seeded', '{}'::jsonb, 'defaults', '{}'::jsonb, 'lg_defaults', '{}'::jsonb, 'listed', '[]'::jsonb, 'issues', '[]'::jsonb, 'errors', 0, 'warns', 0, 'status', 'fix');
BEGIN
  SELECT string_agg(quote_ident(a.attname), ', ') INTO cols FROM pg_catalog.pg_attribute a WHERE a.attrelid = 'public.build_row'::regclass AND a.attnum > 0 AND NOT a.attisdropped AND a.attgenerated = '';
  EXECUTE format('INSERT INTO build_row (%1$s) SELECT %1$s FROM jsonb_array_elements($1) e, LATERAL jsonb_populate_record(NULL::build_row, $2 || jsonb_build_object(''id'', uuidv7()) || e) p', cols) USING rows, base;
END $f$;

CREATE OR REPLACE FUNCTION build_changed() RETURNS trigger @DEFINER@ AS $f$
BEGIN
  IF NEW.seed IS NOT NULL THEN
    UPDATE build SET seed = NULL WHERE id = NEW.id;
    PERFORM build_rows_seed(NEW.id, NEW.seed);
  END IF;
  IF NEW.reset_at IS DISTINCT FROM OLD.reset_at AND NEW.reset_at IS NOT NULL THEN PERFORM build_rows_reset(NEW.id); END IF;
  IF (NEW.restamp_at IS DISTINCT FROM OLD.restamp_at AND NEW.restamp_at IS NOT NULL) OR NEW.photos IS DISTINCT FROM OLD.photos THEN PERFORM build_rows_again(NEW.id); END IF;
  IF NEW.stage = 'complete' AND OLD.stage <> 'complete' THEN
    INSERT INTO product (identifier, name, category, description)
      SELECT DISTINCT ON (lower(CASE WHEN q.sku <> '' THEN q.sku ELSE q.pname END)) CASE WHEN q.sku <> '' THEN q.sku ELSE q.pname END, q.pname, t.name, q.descr
      FROM (SELECT btrim(r.sku) AS sku, go_trim(coalesce(row_values(r) ->> 'vendor_product_name', '')) AS pname, row_values(r) ->> 'vendor_description' AS descr, r.position FROM build_row r WHERE r.build = NEW.id) q
      JOIN template t ON t.id = NEW.template WHERE q.pname <> '' AND btrim(t.name) <> ''
      ORDER BY lower(CASE WHEN q.sku <> '' THEN q.sku ELSE q.pname END), q.position
    ON CONFLICT (lower(identifier)) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description, updated_at = now();
  END IF;
  RETURN NULL;
END $f$;

CREATE OR REPLACE FUNCTION build_numbered() RETURNS trigger @DEFINER@ AS $f$
BEGIN
  IF coalesce(NEW.number, 0) < 1 THEN
    PERFORM pg_advisory_xact_lock(hashtextextended('build_number', 0));
    NEW.number := (SELECT coalesce(max(number), 0) + 1 FROM build);
  END IF;
  RETURN NEW;
END $f$;

CREATE OR REPLACE FUNCTION build_gone() RETURNS trigger @DEFINER@ AS $f$
BEGIN
  IF OLD.stage NOT IN ('draft', 'cancelled', 'complete') THEN RAISE EXCEPTION 'This build is %. Cancel it first so the vendor is told, then delete it.', stage_words(OLD.stage) USING ERRCODE = 'P0001'; END IF;
  RETURN OLD;
END $f$;

CREATE OR REPLACE FUNCTION image_shape() RETURNS trigger @DEFINER@ AS $f$
DECLARE b build; a jsonb;
BEGIN
  IF TG_OP = 'DELETE' THEN
    SELECT * INTO b FROM build WHERE id = OLD.build;
    IF FOUND AND b.stage IN ('complete', 'cancelled') AND pg_trigger_depth() = 1 THEN RAISE EXCEPTION 'This build is % and takes no more changes.', stage_words(b.stage) USING ERRCODE = 'P0001'; END IF;
    RETURN OLD;
  END IF;
  IF NEW.build_row IS NOT DISTINCT FROM OLD.build_row AND NEW.role IS NOT DISTINCT FROM OLD.role THEN RETURN NEW; END IF;
  SELECT * INTO b FROM build WHERE id = NEW.build;
  IF b.stage IN ('complete', 'cancelled') THEN RAISE EXCEPTION 'This build is % and takes no more changes.', stage_words(b.stage) USING ERRCODE = 'P0001'; END IF;
  IF NOT b.photos THEN RAISE EXCEPTION 'This build does not ask for photographs.' USING ERRCODE = 'P0001'; END IF;
  NEW.role := lower(go_trim(coalesce(NEW.role, '')));
  IF NEW.build_row IS NULL THEN NEW.role := ''; RETURN NEW; END IF;
  IF NEW.role = '' THEN NEW.role := 'full'; END IF;
  a := angle_of(NEW.role);
  IF a IS NULL THEN RAISE EXCEPTION '% is not an angle.', NEW.role USING ERRCODE = 'P0001'; END IF;
  IF coalesce((a ->> 'motion')::boolean, false) AND NEW.measured AND NEW.media NOT IN ('gif', 'mp4', 'mov', 'webm') THEN RAISE EXCEPTION 'Only a GIF or a video goes in the Video slot.' USING ERRCODE = 'P0001'; END IF;
  IF NOT coalesce((a ->> 'motion')::boolean, false) AND NEW.media IN ('mp4', 'mov', 'webm') THEN RAISE EXCEPTION 'A video goes in the Video slot.' USING ERRCODE = 'P0001'; END IF;
  IF NOT EXISTS (SELECT FROM build_row r WHERE r.id = NEW.build_row AND r.build = NEW.build) THEN RAISE EXCEPTION 'That is not a product of this build.' USING ERRCODE = 'P0001'; END IF;
  RETURN NEW;
END $f$;

CREATE OR REPLACE FUNCTION image_moved() RETURNS trigger @DEFINER@ AS $f$
DECLARE b build; holds int; spec jsonb; touched uuid[];
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.build_row IS NOT NULL AND EXISTS (SELECT FROM build_row r WHERE r.id = OLD.build_row) THEN
      SELECT * INTO b FROM build WHERE id = OLD.build;
      IF FOUND AND b.stage NOT IN ('complete', 'cancelled') THEN UPDATE build_row SET updated_at = now() WHERE id = OLD.build_row; END IF;
    END IF;
    RETURN NULL;
  END IF;
  IF NEW.build_row IS NOT DISTINCT FROM OLD.build_row AND NEW.role IS NOT DISTINCT FROM OLD.role THEN RETURN NULL; END IF;
  IF NEW.build_row IS NOT NULL AND NOT coalesce((angle_of(NEW.role) ->> 'many')::boolean, false) THEN
    SELECT * INTO b FROM build WHERE id = NEW.build;
    spec := spec_of(b.template);
    SELECT greatest(1, coalesce((SELECT (y ->> 'max')::int FROM jsonb_array_elements(images_owed(spec, row_values(r))) y WHERE y ->> 'role' = NEW.role), 1)) INTO holds FROM build_row r WHERE r.id = NEW.build_row;
    UPDATE build_image i SET build_row = NULL, role = '' WHERE i.build = NEW.build AND i.build_row = NEW.build_row AND i.role = NEW.role AND i.id <> NEW.id
      AND i.id NOT IN (SELECT k.id FROM build_image k WHERE k.build = NEW.build AND k.build_row = NEW.build_row AND k.role = NEW.role AND k.id <> NEW.id ORDER BY k.position LIMIT holds - 1);
  END IF;
  touched := ARRAY(SELECT DISTINCT x FROM unnest(ARRAY[OLD.build_row, NEW.build_row]) x WHERE x IS NOT NULL);
  IF cardinality(touched) > 0 THEN UPDATE build_row SET updated_at = now() WHERE id = ANY(touched); END IF;
  RETURN NULL;
END $f$;

CREATE OR REPLACE FUNCTION vendor_shape() RETURNS trigger @DEFINER@ AS $f$
DECLARE c jsonb; seen text[] := '{}'; out jsonb := '[]'; mail text; base text; s text; n int := 2; other text;
BEGIN
  NEW.name := go_trim(coalesce(NEW.name, ''));
  IF NEW.name = '' THEN RAISE EXCEPTION 'a vendor needs a name' USING ERRCODE = 'P0001'; END IF;
  IF TG_OP = 'INSERT' OR NEW.contacts IS DISTINCT FROM OLD.contacts THEN
    FOR c IN SELECT x FROM jsonb_array_elements(NEW.contacts) x LOOP
      mail := lower(go_trim(coalesce(c ->> 'email', '')));
      IF mail !~ '^[^@\s]+@[^@\s]+\.[^@\s]+$' THEN RAISE EXCEPTION '% is not an email address', go_quote(mail) USING ERRCODE = 'P0001'; END IF;
      IF mail = ANY(seen) THEN CONTINUE; END IF;
      seen := seen || mail;
      out := out || jsonb_build_object('name', go_trim(coalesce(c ->> 'name', '')), 'email', mail);
    END LOOP;
    NEW.contacts := out;
  END IF;
  IF TG_OP = 'INSERT' OR NEW.name IS DISTINCT FROM OLD.name THEN
    SELECT v.name INTO other FROM vendor v WHERE lower(v.name) = lower(NEW.name) AND v.id <> NEW.id LIMIT 1;
    IF other IS NOT NULL THEN RAISE EXCEPTION 'there is already a vendor named %', other USING ERRCODE = 'P0001'; END IF;
    IF TG_OP = 'INSERT' AND NEW.slug <> '' THEN RETURN NEW; END IF;
    base := regexp_replace(regexp_replace(lower(NEW.name), '[^a-z0-9]+', '-', 'g'), '^-+|-+$', '', 'g');
    IF base = '' THEN base := 'vendor'; END IF;
    s := base;
    WHILE EXISTS (SELECT FROM vendor v WHERE v.slug = s AND v.id <> NEW.id) LOOP
      s := base || '-' || n;
      n := n + 1;
    END LOOP;
    NEW.slug := s;
  END IF;
  RETURN NEW;
END $f$;

CREATE OR REPLACE FUNCTION vendor_gone() RETURNS trigger @DEFINER@ AS $f$
DECLARE n int := (SELECT count(*) FROM build b WHERE b.vendor = OLD.id);
BEGIN
  IF n > 0 THEN RAISE EXCEPTION '% still has % %. Delete or cancel those first.', OLD.name, n, CASE WHEN n = 1 THEN 'build' ELSE 'builds' END USING ERRCODE = 'P0001'; END IF;
  RETURN OLD;
END $f$;

CREATE OR REPLACE FUNCTION template_shape() RETURNS trigger @DEFINER@ AS $f$
DECLARE base text; k text; n int := 2; a template;
BEGIN
  NEW.name := go_trim(coalesce(NEW.name, ''));
  IF NEW.name = '' THEN RAISE EXCEPTION 'a template needs a name' USING ERRCODE = 'P0001'; END IF;
  IF TG_OP = 'INSERT' THEN
    IF NEW.key = '' THEN
      base := regexp_replace(regexp_replace(lower(NEW.name), '[^a-z0-9]+', '_', 'g'), '^_+|_+$', '', 'g');
      IF base = '' THEN base := 'template'; END IF;
      k := base;
      WHILE EXISTS (SELECT FROM template t WHERE t.key = k) LOOP
        k := base || '_' || n;
        n := n + 1;
      END LOOP;
      NEW.key := k;
      IF NEW.parent IS NULL THEN NEW.parent := (SELECT t.id FROM template t WHERE t.parent IS NULL LIMIT 1); END IF;
    END IF;
    RETURN NEW;
  END IF;
  IF NEW.key IS DISTINCT FROM OLD.key THEN RAISE EXCEPTION 'the key changes only through publish_template' USING ERRCODE = 'P0001'; END IF;
  IF NEW.parent IS DISTINCT FROM OLD.parent THEN
    IF NEW.parent IS NULL THEN RAISE EXCEPTION '% inherits from another template; only the root stands alone', OLD.name USING ERRCODE = 'P0001'; END IF;
    IF NEW.parent = NEW.id THEN RAISE EXCEPTION '% cannot inherit from itself', OLD.name USING ERRCODE = 'P0001'; END IF;
    SELECT * INTO a FROM template t WHERE t.id = NEW.parent;
    WHILE FOUND LOOP
      IF a.id = NEW.id THEN RAISE EXCEPTION '% already inherits from %; the two would inherit from each other', (SELECT t.name FROM template t WHERE t.id = NEW.parent), OLD.name USING ERRCODE = 'P0001'; END IF;
      EXIT WHEN a.parent IS NULL;
      SELECT * INTO a FROM template t WHERE t.id = a.parent;
    END LOOP;
  END IF;
  RETURN NEW;
END $f$;

CREATE OR REPLACE FUNCTION template_gone() RETURNS trigger @DEFINER@ AS $f$
DECLARE n int; kids text;
BEGIN
  IF OLD.parent IS NULL THEN RAISE EXCEPTION '% is where every template starts; it stays.', OLD.name USING ERRCODE = 'P0001'; END IF;
  n := (SELECT count(*) FROM build b WHERE b.template = OLD.id);
  IF n > 0 THEN RAISE EXCEPTION '% still has % % reading it. Delete those first.', OLD.name, n, CASE WHEN n = 1 THEN 'build' ELSE 'builds' END USING ERRCODE = 'P0001'; END IF;
  SELECT string_agg(t.name, ', '), count(*) INTO kids, n FROM template t WHERE t.parent = OLD.id;
  IF n > 0 THEN RAISE EXCEPTION '% % from this template. Move or delete those first.', kids, CASE WHEN n = 1 THEN 'inherits' ELSE 'inherit' END USING ERRCODE = 'P0001'; END IF;
  RETURN OLD;
END $f$;

CREATE OR REPLACE FUNCTION rule_shape() RETURNS trigger @DEFINER@ AS $f$
DECLARE known text[]; hit text; method text := go_trim(coalesce(NEW.ship_method, ''));
BEGIN
  IF TG_OP = 'UPDATE' AND NEW.vendor IS DISTINCT FROM OLD.vendor THEN RAISE EXCEPTION 'A lead time stays with its vendor; delete it and add one on the other vendor.' USING ERRCODE = 'P0001'; END IF;
  IF NEW.max_time < NEW.min_time THEN RAISE EXCEPTION 'The longest time is shorter than the shortest.' USING ERRCODE = 'P0001'; END IF;
  IF method <> '' THEN
    known := ARRAY(SELECT DISTINCT o FROM field f, unnest(f.options) o WHERE f.type = 'select' AND ((f.owner = 'lg' AND (f.key ~* 'ship(ping)?[ _-]*class' OR f.name ~* 'ship(ping)?[ _-]*class')) OR f.key ~* 'ship(ping)?[ _-]*(method|via)' OR f.name ~* 'ship(ping)?[ _-]*(method|via)'));
    hit := (SELECT o FROM unnest(known) o WHERE lower(o) = lower(method) LIMIT 1);
    IF hit IS NULL THEN RAISE EXCEPTION '% is not a ship method the templates offer (%).', method, array_to_string(known, ', ') USING ERRCODE = 'P0001'; END IF;
    NEW.ship_method := hit;
  ELSE
    NEW.ship_method := NULL;
  END IF;
  NEW.name := coalesce(NEW.ship_method, 'Any method') || ': ' || lead_text(NEW.min_time, NEW.max_time, NEW.unit::text);
  RETURN NEW;
END $f$;

CREATE OR REPLACE FUNCTION rule_changed() RETURNS trigger @DEFINER@ AS $f$
DECLARE vid uuid := CASE WHEN TG_OP = 'DELETE' THEN OLD.vendor ELSE NEW.vendor END; bid uuid;
BEGIN
  FOR bid IN SELECT b.id FROM build b WHERE b.vendor = vid AND b.stage NOT IN ('complete', 'cancelled') LOOP
    PERFORM build_rows_again(bid);
  END LOOP;
  RETURN NULL;
END $f$;
`.replaceAll('@DEFINER@', DEFINER);

const TRIGGERS = String.raw`
CREATE OR REPLACE TRIGGER stamp BEFORE INSERT OR UPDATE ON build_row FOR EACH ROW EXECUTE FUNCTION build_row_stamp();
CREATE OR REPLACE TRIGGER counted_insert AFTER INSERT ON build_row REFERENCING NEW TABLE AS changed FOR EACH STATEMENT EXECUTE FUNCTION build_row_counted();
CREATE OR REPLACE TRIGGER counted_update AFTER UPDATE ON build_row REFERENCING NEW TABLE AS changed FOR EACH STATEMENT EXECUTE FUNCTION build_row_counted();
CREATE OR REPLACE TRIGGER counted_delete AFTER DELETE ON build_row REFERENCING OLD TABLE AS changed FOR EACH STATEMENT EXECUTE FUNCTION build_row_counted();
CREATE OR REPLACE TRIGGER numbered BEFORE INSERT ON build FOR EACH ROW EXECUTE FUNCTION build_numbered();
CREATE OR REPLACE TRIGGER changed AFTER UPDATE ON build FOR EACH ROW EXECUTE FUNCTION build_changed();
CREATE OR REPLACE TRIGGER gone BEFORE DELETE ON build FOR EACH ROW EXECUTE FUNCTION build_gone();
CREATE OR REPLACE TRIGGER shape BEFORE UPDATE OR DELETE ON build_image FOR EACH ROW EXECUTE FUNCTION image_shape();
CREATE OR REPLACE TRIGGER moved AFTER UPDATE OR DELETE ON build_image FOR EACH ROW EXECUTE FUNCTION image_moved();
CREATE OR REPLACE TRIGGER shape BEFORE INSERT OR UPDATE ON vendor FOR EACH ROW EXECUTE FUNCTION vendor_shape();
CREATE OR REPLACE TRIGGER gone BEFORE DELETE ON vendor FOR EACH ROW EXECUTE FUNCTION vendor_gone();
CREATE OR REPLACE TRIGGER shape BEFORE INSERT OR UPDATE ON template FOR EACH ROW EXECUTE FUNCTION template_shape();
CREATE OR REPLACE TRIGGER gone BEFORE DELETE ON template FOR EACH ROW EXECUTE FUNCTION template_gone();
CREATE OR REPLACE TRIGGER shape BEFORE INSERT OR UPDATE ON lead_time_rule FOR EACH ROW EXECUTE FUNCTION rule_shape();
CREATE OR REPLACE TRIGGER changed AFTER INSERT OR UPDATE OR DELETE ON lead_time_rule FOR EACH ROW EXECUTE FUNCTION rule_changed();
`;

const VIEWS = String.raw`
CREATE OR REPLACE FUNCTION photo_label(field text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $f$
  SELECT CASE WHEN field = 'photo' THEN 'A placed photo that fails the photo spec' WHEN field = 'image:video' THEN 'Video or GIF' WHEN field = 'image:three_quarter' THEN '3/4 angle photo' WHEN field LIKE 'image:%' THEN initcap(replace(substr(field, 7), '_', ' ')) || ' photo' END
$f$;

CREATE OR REPLACE VIEW build_book WITH (security_invoker = true) AS
SELECT b.id, b.number, b.name, v.name AS vendor, v.slug AS vendor_slug, b.vendor AS vendor_id, coalesce(t.key, '') AS category, coalesce(t.name, '') AS category_name, coalesce(t.icon, '') AS category_icon, b.template, b.stage,
  CASE WHEN b.stage IN ('sent', 'in_progress') AND b.due_at < now() THEN 'overdue' ELSE b.stage END AS status,
  CASE WHEN b.stage = 'draft' THEN 'Draft' WHEN b.stage = 'review' THEN 'Review' WHEN b.stage = 'complete' THEN 'Approved' WHEN b.stage = 'cancelled' THEN 'Cancelled' WHEN b.due_at < now() AND b.opened_at IS NULL THEN 'Overdue, never opened' WHEN b.due_at < now() THEN 'Overdue' WHEN b.stage = 'in_progress' THEN 'In progress' ELSE 'Sent' END AS standing,
  CASE WHEN b.stage = 'review' THEN 'amber' WHEN b.stage = 'complete' THEN 'green' WHEN b.stage = 'cancelled' THEN 'neutral' WHEN b.stage IN ('sent', 'in_progress') AND b.due_at < now() THEN 'red' WHEN b.stage = 'in_progress' THEN 'blue' ELSE 'neutral' END AS tone,
  CASE WHEN b.stage = 'complete' THEN 'Approved' WHEN b.stage = 'review' THEN 'Submitted' WHEN b.opened_at IS NOT NULL THEN 'Opened' WHEN b.sent_at IS NOT NULL THEN 'Sent' ELSE 'Draft' END AS step,
  b.clean || ' of ' || b.skus AS progress, b.skus, b.clean,
  (SELECT count(*) FROM build_row r WHERE r.build = b.id AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(r.issues) i WHERE i ->> 'level' = 'error' AND i ->> 'field' !~ '^(image|photo):'))::int AS data_clean,
  CASE WHEN b.photos THEN (SELECT count(*) FROM build_row r WHERE r.build = b.id AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(r.issues) i WHERE i ->> 'level' = 'error' AND i ->> 'field' ~ '^(image|photo):'))::int END AS photos_clean,
  b.photos AS wants_photos,
  CASE WHEN b.photos THEN (SELECT count(*) FROM build_image i WHERE i.build = b.id) ELSE 0 END AS photos,
  CASE WHEN b.photos THEN (SELECT count(*) FROM build_image i WHERE i.build = b.id AND i.build_row IS NOT NULL) ELSE 0 END AS photos_placed,
  b.errors, b.warns, b.emails, b.created_by, b.created_by_name, b.due_at, b.remind_every_days, b.reminders_sent, b.sent_at, b.opened_at, b.submitted_at, b.returned_at, b.approved_at, b.cancelled_at,
  coalesce((SELECT max(a.at) FROM build_activity a WHERE a.build = b.id), b.created_at) AS last_activity_at,
  (SELECT max(a.at) FROM build_activity a WHERE a.build = b.id AND a.who = 'vendor' AND a.text <> 'Opened the link') AS vendor_active_at,
  b.created_at
FROM build b JOIN vendor v ON v.id = b.vendor LEFT JOIN template t ON t.id = b.template ORDER BY b.created_at DESC;

CREATE OR REPLACE VIEW portal_book WITH (security_invoker = true) AS
SELECT b.id, b.number, b.name, v.name AS vendor, b.vendor AS vendor_id, coalesce(t.key, '') AS category, coalesce(t.name, '') AS category_name, coalesce(t.icon, '') AS category_icon, b.template, b.stage, b.skus, b.clean,
  (SELECT count(*) FROM build_row r WHERE r.build = b.id AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(r.issues) i WHERE i ->> 'level' = 'error' AND i ->> 'field' !~ '^(image|photo):'))::int AS data_clean,
  CASE WHEN b.photos THEN (SELECT count(*) FROM build_row r WHERE r.build = b.id AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(r.issues) i WHERE i ->> 'level' = 'error' AND i ->> 'field' ~ '^(image|photo):'))::int END AS photos_clean,
  b.photos AS wants_photos,
  CASE WHEN b.photos THEN (SELECT count(*) FROM build_image i WHERE i.build = b.id) ELSE 0 END AS photos,
  CASE WHEN b.photos THEN (SELECT count(*) FROM build_image i WHERE i.build = b.id AND i.build_row IS NOT NULL) ELSE 0 END AS photos_placed,
  b.errors, b.warns, b.due_at, b.sent_at, b.opened_at, b.submitted_at, b.returned_at, b.return_note, b.approved_at, b.cancelled_at, b.source, b.created_at
FROM build b JOIN vendor v ON v.id = b.vendor LEFT JOIN template t ON t.id = b.template ORDER BY b.number;
GRANT SELECT (id, number, name, vendor, vendor_id, category, category_name, category_icon, template, stage, skus, clean, data_clean, photos_clean, wants_photos, photos, photos_placed, errors, warns, due_at, sent_at, opened_at, submitted_at, returned_at, return_note, approved_at, cancelled_at, source, created_at) ON portal_book TO guest;

CREATE OR REPLACE VIEW vendor_book WITH (security_invoker = true) AS
SELECT v.id, v.name, v.slug, v.country, v.contacts, jsonb_array_length(v.contacts)::int AS contact_count,
  (SELECT count(*) FROM build b WHERE b.vendor = v.id AND b.stage NOT IN ('draft', 'complete', 'cancelled'))::int AS open_builds,
  (SELECT count(*) FROM build b WHERE b.vendor = v.id)::int AS builds,
  (SELECT max(b.sent_at) FROM build b WHERE b.vendor = v.id) AS last_sent_at,
  v.link_at IS NOT NULL AS has_link, v.link_at AS link_minted_at, v.created_at
FROM vendor v ORDER BY v.name;

CREATE OR REPLACE VIEW template_book WITH (security_invoker = true) AS
SELECT t.id, t.key, t.name, t.parent, p.key AS parent_key, p.name AS parent_name, t.icon, t.sheet, t.shell,
  (SELECT count(*) FROM build b WHERE b.template = t.id AND b.stage NOT IN ('complete', 'cancelled'))::int AS open_builds,
  (SELECT count(*) FROM template c WHERE c.parent = t.id)::int AS children, t.updated_at, t.created_at
FROM template t LEFT JOIN template p ON p.id = t.parent ORDER BY t.name;

CREATE OR REPLACE VIEW build_owed WITH (security_invoker = true) AS
WITH RECURSIVE started AS (
  SELECT b.id AS build, EXISTS (SELECT 1 FROM build_row r WHERE r.build = b.id AND row_values(r) <> r.seeded || r.lg_seeded) OR EXISTS (SELECT 1 FROM build_image i WHERE i.build = b.id) AS started
  FROM build b WHERE b.stage NOT IN ('complete', 'cancelled')
), chain AS (
  SELECT b.id AS build, t.id AS template, t.parent, 0 AS depth FROM build b JOIN template t ON t.id = b.template WHERE b.stage NOT IN ('complete', 'cancelled')
  UNION ALL SELECT c.build, p.id, p.parent, c.depth + 1 FROM chain c JOIN template p ON p.id = c.parent WHERE c.depth < 16
), labels AS (
  SELECT DISTINCT ON (c.build, f.key) c.build, f.key, f.name AS label FROM chain c JOIN field f ON f.template = c.template ORDER BY c.build, f.key, c.depth
), owed AS (
  SELECT r.build, r.id AS row_id, r.sku, coalesce(nullif(r.name, ''), '') AS product, CASE WHEN i ->> 'field' LIKE 'photo:%' THEN 'photo' ELSE i ->> 'field' END AS field, i ->> 'level' AS level, i ->> 'message' AS message
  FROM build_row r JOIN started s ON s.build = r.build AND s.started CROSS JOIN LATERAL jsonb_array_elements(r.issues) i
)
SELECT md5(o.build::text || '|' || o.field || '|' || o.level)::uuid AS id, o.build, b.number AS build_number, b.name AS build_name, v.name AS vendor, b.vendor AS vendor_id,
  CASE WHEN o.field LIKE 'image:%' OR o.field = 'photo' THEN 'photo' ELSE 'data' END AS kind, o.field, coalesce(photo_label(o.field), nullif(l.label, ''), o.field) AS label, o.level,
  cardinality(array_agg(DISTINCT o.row_id)) AS products, array_agg(DISTINCT o.row_id) AS rows, array_remove(array_agg(DISTINCT o.sku ORDER BY o.sku), '') AS skus,
  array_agg(DISTINCT btrim(o.sku || ' ' || o.product) ORDER BY btrim(o.sku || ' ' || o.product)) AS items, (array_agg(DISTINCT o.message))[1:5] AS messages, coalesce(photo_label(o.field), nullif(l.label, ''), o.field) AS name
FROM owed o JOIN build b ON b.id = o.build JOIN vendor v ON v.id = b.vendor LEFT JOIN labels l ON l.build = o.build AND l.key = o.field
GROUP BY o.build, b.number, b.name, v.name, b.vendor, o.field, o.level, l.label
UNION ALL
SELECT md5(b.id::text || '|not_started')::uuid, b.id, b.number, b.name, v.name, b.vendor, 'all', 'not_started', 'Not started: nothing entered and no photo yet', 'error', cardinality(array_agg(r.id)), array_agg(r.id), array_remove(array_agg(r.sku ORDER BY r.sku), ''), array_agg(btrim(r.sku || ' ' || coalesce(nullif(r.name, ''), '')) ORDER BY r.sku, r.name), ARRAY['Every required field and photograph'], 'Not started: nothing entered and no photo yet'
FROM started s JOIN build b ON b.id = s.build JOIN vendor v ON v.id = b.vendor JOIN build_row r ON r.build = b.id WHERE NOT s.started GROUP BY b.id, b.number, b.name, v.name, b.vendor;

CREATE OR REPLACE VIEW build_sku WITH (security_invoker = true) AS
WITH owed AS (
  SELECT o.build, s.row_id, array_agg(o.label ORDER BY o.kind, o.label) FILTER (WHERE o.level = 'error') AS owes, array_agg(o.label ORDER BY o.label) FILTER (WHERE o.level <> 'error') AS notes
  FROM build_owed o CROSS JOIN LATERAL unnest(o.rows) s(row_id) GROUP BY o.build, s.row_id
)
SELECT r.id, r.name, r.build, b.number AS build_number, b.name AS build_name, v.name AS vendor, r.sku, coalesce(nullif(r.name, ''), '') AS product, r.status <> 'fix' AS done, r.errors, r.warns, coalesce(w.owes, '{}') AS owes, coalesce(w.notes, '{}') AS notes,
  coalesce(ARRAY(SELECT i ->> 'message' FROM jsonb_array_elements(r.issues) i WHERE i ->> 'level' = 'error'), '{}') AS why,
  (SELECT count(*) FROM build_image i WHERE i.build_row = r.id)::int AS photos,
  cardinality(ARRAY(SELECT DISTINCT x.build FROM build_row x JOIN build o ON o.id = x.build WHERE o.stage <> 'cancelled' AND (CASE WHEN btrim(r.sku) <> '' THEN lower(btrim(x.sku)) = lower(btrim(r.sku)) ELSE btrim(x.sku) = '' AND lower(btrim(x.name)) = lower(btrim(r.name)) END))) AS on_builds,
  coalesce(row_values(r) ->> 'lead_time', '') AS lead_time, coalesce(row_values(r) ->> 'shipping_method', '') AS ship_method, r.position
FROM build_row r JOIN build b ON b.id = r.build JOIN vendor v ON v.id = b.vendor LEFT JOIN owed w ON w.build = r.build AND w.row_id = r.id WHERE b.stage <> 'cancelled';

CREATE OR REPLACE VIEW sku_elsewhere WITH (security_invoker = true) AS
SELECT md5(r.id::text || x.id::text)::uuid AS id, r.build, r.sku, r.name, o.id AS other_build, o.number AS other_number, o.name AS other_name, v.name AS other_vendor, o.stage AS other_stage
FROM build_row r JOIN build_row x ON x.build <> r.build AND (CASE WHEN btrim(r.sku) <> '' THEN lower(btrim(x.sku)) = lower(btrim(r.sku)) ELSE btrim(x.sku) = '' AND lower(btrim(x.name)) = lower(btrim(r.name)) END)
JOIN build o ON o.id = x.build JOIN vendor v ON v.id = o.vendor WHERE o.stage <> 'cancelled';
`;

const MORE = String.raw`
ALTER TABLE intake ADD COLUMN synonyms jsonb NOT NULL DEFAULT '{}';
COMMENT ON COLUMN intake.synonyms IS 'The header words a buying assistant recognises at a glance for each field key.';
`;

const MORE3 = String.raw`
ALTER TABLE build ADD COLUMN seed jsonb;
COMMENT ON COLUMN build.seed IS 'Written with [{position, sku, name, <field>: value}] to add those products in one statement; empty again once they are in.';
`;

const PRE = String.raw`
const I = (typeof input === "object" && input !== null && !Array.isArray(input) ? input : {}) as Record<string, unknown>;
const text = (k: string): string => { const v = I[k]; return typeof v === "string" ? v.trim() : ""; };
const list = (k: string): string[] => { const v = I[k]; return Array.isArray(v) ? v.map(x => String(x ?? "").trim()).filter(x => x !== "") : []; };
const refuse = (m: string): never => { throw new Error(m); };
const buildRef = (): string => text("id") || text("build");
const words = (s: string): string => ({ draft: "a draft", sent: "sent and not yet opened", in_progress: "in progress with the vendor", review: "in review", complete: "approved", cancelled: "cancelled" } as Record<string, string>)[s] ?? s;
const plural = (n: number): string => n === 1 ? "" : "s";
const count = (n: number, one: string): string => n + " " + one + (n === 1 ? "" : "s");
const now = (): string => new Date().toISOString();
const who = (): string => caller.scope ? "vendor" : "ba";
const loadBuild = async () => { const id = buildRef(); if (id === "") refuse("id is required"); try { return await archive.get("build", id); } catch { return refuse("no such build"); } };
const openOf = async () => { const b = await loadBuild(); if (b.stage === "complete" || b.stage === "cancelled") refuse("This build is " + words(b.stage) + " and takes no more changes."); return b; };
const line = async (build: string, by: string, t: string): Promise<void> => { await archive.create("build_activity", { build, who: by, text: t, name: t } as never); };
const mail = async (kind: string, extra: Row): Promise<string> => { const r = await archive.run("mail", { kind, ...extra }); if (r.error !== "") return r.error; const o = r.result as { mail_error?: string } | null; return o?.mail_error ?? ""; };
const made = <T>(x: unknown): T => x as T;
const fold = (s: string): string => s.trim().toLowerCase();
const squash = (s: string): string => s.toLowerCase().replace(/[^a-z0-9]/g, "");
const safeName = (s: string): string => s.replace(/[^a-zA-Z0-9_\- ]/g, "").trim().replace(/ +/g, "_") || "build";
const listedAs = (sku: string, name: string): string => sku.trim() || name.trim();
type Spec = { key: string; name: string; icon: string; sheet: string; shell: string | null; fields: Field[]; images: { role: string; min: number; max?: number; when?: unknown }[] | null; photo: Row | null; export: { row?: number; columns?: Record<string, string>; compose?: Record<string, { parts?: { prefix?: string; suffix?: string }[]; join?: string }> } };
type Field = { key: string; label: string; type: string; section: string; owner: string; required?: boolean; requiredWhen?: unknown[]; options?: string[]; parts?: string[]; help?: string; multi?: boolean; pctBreakdown?: boolean };
const specOf = async (template: string | null): Promise<Spec> => { if (template === null) return refuse("this build has no template"); const r = await archive.query("SELECT template_spec($1::uuid) AS spec", template); return r[0]!.spec as Spec; };
const rowsOf = async (build: string) => (await archive.query("SELECT r.id, r.sku, r.name, r.position, row_values(r) AS v, r.listed FROM build_row r WHERE r.build = $1::uuid ORDER BY r.position, r.id", build)).map(r => ({ id: r.id as string, sku: r.sku as string, name: r.name as string, position: r.position as number, v: r.v as Record<string, string>, listed: (r.listed ?? []) as string[] }));
const elsewhere = async (skus: string[], names: string[], exclude: string) => {
  if (skus.length + names.length === 0) return [];
  const r = await archive.query("SELECT r.sku, r.name AS product, b.id AS build, b.number, b.name, v.name AS vendor, b.stage FROM build_row r JOIN build b ON b.id = r.build JOIN vendor v ON v.id = b.vendor WHERE b.stage <> 'cancelled' AND (lower(btrim(r.sku)) = ANY($1::text[]) OR (btrim(r.sku) = '' AND lower(btrim(r.name)) = ANY($2::text[]))) AND ($3::text = '' OR b.id::text <> $3::text) ORDER BY b.number, r.position", skus.map(fold), names.map(fold), exclude);
  return r;
};
`;

const ICONS_DATA = {"bed": "<path d=\"M2 4v16\" /> <path d=\"M2 8h18a2 2 0 0 1 2 2v10\" /> <path d=\"M2 17h20\" /> <path d=\"M6 8v9\" />", "blinds": "<path d=\"M3 3h18\" /> <path d=\"M20 7H8\" /> <path d=\"M20 11H8\" /> <path d=\"M10 19h10\" /> <path d=\"M8 15h12\" /> <path d=\"M4 3v14\" /> <circle cx=\"4\" cy=\"19\" r=\"2\" />", "box": "<path d=\"M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z\" /> <path d=\"m3.3 7 8.7 5 8.7-5\" /> <path d=\"M12 22V12\" />", "brush": "<path d=\"m11 10 3 3\" /> <path d=\"M6.5 21A3.5 3.5 0 1 0 3 17.5a2.62 2.62 0 0 1-.708 1.792A1 1 0 0 0 3 21z\" /> <path d=\"M9.969 17.031 21.378 5.624a1 1 0 0 0-3.002-3.002L6.967 14.031\" />", "camera": "<path d=\"M13.997 4a2 2 0 0 1 1.76 1.05l.486.9A2 2 0 0 0 18.003 7H20a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2h1.997a2 2 0 0 0 1.759-1.048l.489-.904A2 2 0 0 1 10.004 4z\" /> <circle cx=\"12\" cy=\"13\" r=\"3\" />", "clock": "<circle cx=\"12\" cy=\"12\" r=\"10\" /> <path d=\"M12 6v6l4 2\" />", "fan": "<path d=\"M10.827 16.379a6.082 6.082 0 0 1-8.618-7.002l5.412 1.45a6.082 6.082 0 0 1 7.002-8.618l-1.45 5.412a6.082 6.082 0 0 1 8.618 7.002l-5.412-1.45a6.082 6.082 0 0 1-7.002 8.618l1.45-5.412Z\" /> <path d=\"M12 12v.01\" />", "frame": "<line x1=\"22\" x2=\"2\" y1=\"6\" y2=\"6\" /> <line x1=\"22\" x2=\"2\" y1=\"18\" y2=\"18\" /> <line x1=\"6\" x2=\"6\" y1=\"2\" y2=\"22\" /> <line x1=\"18\" x2=\"18\" y1=\"2\" y2=\"22\" />", "gift": "<path d=\"M12 7v14\" /> <path d=\"M20 11v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-8\" /> <path d=\"M7.5 7a1 1 0 0 1 0-5A4.8 8 0 0 1 12 7a4.8 8 0 0 1 4.5-5 1 1 0 0 1 0 5\" /> <rect x=\"3\" y=\"7\" width=\"18\" height=\"4\" rx=\"1\" />", "globe": "<circle cx=\"12\" cy=\"12\" r=\"10\" /> <path d=\"M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20\" /> <path d=\"M2 12h20\" />", "house": "<path d=\"M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8\" /> <path d=\"M3 10a2 2 0 0 1 .709-1.528l7-6a2 2 0 0 1 2.582 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z\" />", "image": "<rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\" ry=\"2\" /> <circle cx=\"9\" cy=\"9\" r=\"2\" /> <path d=\"m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21\" />", "lamp": "<path d=\"M12 12v6\" /> <path d=\"M4.077 10.615A1 1 0 0 0 5 12h14a1 1 0 0 0 .923-1.385l-3.077-7.384A2 2 0 0 0 15 2H9a2 2 0 0 0-1.846 1.23Z\" /> <path d=\"M8 20a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v1a1 1 0 0 1-1 1H9a1 1 0 0 1-1-1z\" />", "layout-dashboard": "<rect width=\"7\" height=\"9\" x=\"3\" y=\"3\" rx=\"1\" /> <rect width=\"7\" height=\"5\" x=\"14\" y=\"3\" rx=\"1\" /> <rect width=\"7\" height=\"9\" x=\"14\" y=\"12\" rx=\"1\" /> <rect width=\"7\" height=\"5\" x=\"3\" y=\"16\" rx=\"1\" />", "moon": "<path d=\"M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401\" />", "palette": "<path d=\"M12 22a1 1 0 0 1 0-20 10 9 0 0 1 10 9 5 5 0 0 1-5 5h-2.25a1.75 1.75 0 0 0-1.4 2.8l.3.4a1.75 1.75 0 0 1-1.4 2.8z\" /> <circle cx=\"13.5\" cy=\"6.5\" r=\".5\" fill=\"currentColor\" /> <circle cx=\"17.5\" cy=\"10.5\" r=\".5\" fill=\"currentColor\" /> <circle cx=\"6.5\" cy=\"12.5\" r=\".5\" fill=\"currentColor\" /> <circle cx=\"8.5\" cy=\"7.5\" r=\".5\" fill=\"currentColor\" />", "rectangle-horizontal": "<rect width=\"20\" height=\"12\" x=\"2\" y=\"6\" rx=\"2\" />", "rectangle-vertical": "<rect width=\"12\" height=\"20\" x=\"6\" y=\"2\" rx=\"2\" />", "settings": "<path d=\"M9.671 4.136a2.34 2.34 0 0 1 4.659 0 2.34 2.34 0 0 0 3.319 1.915 2.34 2.34 0 0 1 2.33 4.033 2.34 2.34 0 0 0 0 3.831 2.34 2.34 0 0 1-2.33 4.033 2.34 2.34 0 0 0-3.319 1.915 2.34 2.34 0 0 1-4.659 0 2.34 2.34 0 0 0-3.32-1.915 2.34 2.34 0 0 1-2.33-4.033 2.34 2.34 0 0 0 0-3.831A2.34 2.34 0 0 1 6.35 6.051a2.34 2.34 0 0 0 3.319-1.915\" /> <circle cx=\"12\" cy=\"12\" r=\"3\" />", "sofa": "<path d=\"M20 9V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v3\" /> <path d=\"M2 16a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-5a2 2 0 0 0-4 0v1.5a.5.5 0 0 1-.5.5h-11a.5.5 0 0 1-.5-.5V11a2 2 0 0 0-4 0z\" /> <path d=\"M4 18v2\" /> <path d=\"M20 18v2\" /> <path d=\"M12 4v9\" />", "square-round-corner": "<path d=\"M21 11a8 8 0 0 0-8-8\" /> <path d=\"M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4\" />", "star": "<path d=\"M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z\" />", "sun": "<circle cx=\"12\" cy=\"12\" r=\"4\" /> <path d=\"M12 2v2\" /> <path d=\"M12 20v2\" /> <path d=\"m4.93 4.93 1.41 1.41\" /> <path d=\"m17.66 17.66 1.41 1.41\" /> <path d=\"M2 12h2\" /> <path d=\"M20 12h2\" /> <path d=\"m6.34 17.66-1.41 1.41\" /> <path d=\"m19.07 4.93-1.41 1.41\" />"};

const MAIL = String.raw`
const C = { ground: "#f3f3f3", paper: "#ffffff", ink: "#111111", soft: "#5f5f5f", rule: "#eaeaea", headFill: "#f5f5f5", hover: "#dedede", flag: "#d92d20", brandFill: "#111111", onBrand: "#fafafa", face: "'Geist', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif", body: "13.5px", line: "20px", cellLine: "18px", title: "34px", titleLine: "44px", tracking: "-0.51px", weight: "400", radius: "6px", radiusBtn: "4px", btnHeight: "28px", s1h: "6px", s2: "8px", s3: "12px", s4: "16px", s6: "24px", s8: "32px" };
const ICONS = JSON.parse(ICONS_TEXT) as Record<string, string>;
const NAME_FILES: Record<string, string> = { product: "box", home: "house", gift: "gift", moon: "moon", sun: "sun", curtain: "blinds", package: "box", bulb: "lamp", pillow: "square-round-corner", rug: "rectangle-horizontal", image: "image", star: "star", brush: "brush", colour: "palette", flower: "fan", heart: "star", globe: "globe", camera: "camera", clock: "clock", ruler: "layout-dashboard", theme: "palette", wrench: "settings" };
const esc = (s: string): string => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&#34;").replace(/'/g, "&#39;");
const cellPadding = C.s2 + " " + C.s3;
const cellType = "font-family:" + C.face + ";font-size:" + C.body + ";font-weight:" + C.weight + ";line-height:" + C.cellLine + ";font-variant-numeric:tabular-nums;";
const icon = (name: string): string => { const f = NAME_FILES[name] ?? name; const inner = ICONS[f] ?? ICONS["box"] ?? ""; return '<svg style="display:block;" xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="' + C.soft + '" stroke-width="0.73" stroke-linecap="round" stroke-linejoin="round">' + inner + "</svg>"; };
type L = { id: string; number: number; name: string; stage: string; photos: boolean; total: number; clean: number; data: number; shots: number; category: string; mark: string; late: boolean; due: string; submitted: string; approved: string; url: string };
const stateOf = (l: L): { due: string; late: boolean } => { if (l.stage === "review") return { due: "Submitted " + l.submitted, late: false }; if (l.stage === "complete") return { due: "Approved " + l.approved, late: false }; return { due: (l.late ? "Overdue " : "Due ") + l.due, late: l.late }; };
const th = (label: string, align: string): string => '<th align="' + align + '" style="padding:' + cellPadding + ";text-align:" + align + ";white-space:nowrap;background:" + C.headFill + ";border-bottom:1px solid " + C.rule + ";color:" + C.ink + ";" + cellType + '">' + label + "</th>";
const rowHtml = (l: L, last: boolean): string => {
  const s = stateOf(l);
  const under = last ? "" : "border-bottom:1px solid " + C.rule + ";";
  const a = (inner: string, colour: string): string => '<a href="' + l.url + '" style="display:block;text-decoration:none;color:' + colour + ";" + cellType + '">' + inner + "</a>";
  const td = (inner: string): string => '<td align="right" valign="middle" style="padding:' + cellPadding + ";vertical-align:middle;text-align:right;white-space:nowrap;" + under + '">' + inner + "</td>";
  const build = '<a href="' + l.url + '" style="display:block;text-decoration:none;color:' + C.ink + ';"><table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr><td class="mark" valign="middle" style="padding-right:10px;vertical-align:middle;line-height:0;">' + icon(l.mark) + '</td><td valign="middle" style="vertical-align:middle;color:' + C.ink + ";" + cellType + '">' + esc(l.name) + '<br><span style="color:' + C.soft + ';">' + esc(l.category) + "</span></td></tr></table></a>";
  return '<tr class="row"><td valign="middle" width="100%" style="padding:' + cellPadding + ";vertical-align:middle;width:100%;" + under + '">' + build + "</td>" + td(a(l.data + "/" + l.total, C.ink)) + td(a(l.shots >= 0 ? l.shots + "/" + l.total : "", C.ink)) + td(a(esc(s.due), s.late ? C.flag : C.soft)) + "</tr>";
};
type Page = { brand: string; first: string; lead: string; quote?: string; lines: L[]; cta: string; url: string; footer: string };
const shell = (p: Page, support: string): string => {
  const rows = p.lines.map((l, i) => rowHtml(l, i === p.lines.length - 1)).join("");
  const card = p.lines.length === 0 ? "" : '<tr><td style="padding-top:' + C.s4 + ';"><div style="background:' + C.paper + ";border:1px solid " + C.rule + ";border-radius:" + C.radius + ';overflow:hidden;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" class="builds" style="width:100%;border-collapse:separate;border-spacing:0;background:' + C.paper + ';"><tr>' + th("Build", "left") + th("Data", "right") + th("Photos", "right") + th("When", "right") + "</tr>" + rows + "</table></div></td></tr>";
  const quote = !p.quote ? "" : '<tr><td style="padding-top:' + C.s4 + ';"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="width:100%;background:' + C.paper + ";border:1px solid " + C.rule + ";border-radius:" + C.radius + ';"><tr><td style="padding:' + cellPadding + ";color:" + C.ink + ";font-family:" + C.face + ";font-size:" + C.body + ";font-weight:" + C.weight + ";line-height:" + C.line + ';">“' + esc(p.quote) + "”</td></tr></table></td></tr>";
  const footer = esc(p.footer).replace(support, '<a href="mailto:' + support + '" style="color:' + C.soft + ';">' + support + "</a>");
  const type2 = "font-family:" + C.face + ";font-size:" + C.body + ";font-weight:" + C.weight + ";line-height:" + C.line + ";color:" + C.soft + ";";
  return '<!doctype html>\n<html lang="en">\n<head>\n<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">\n<meta name="color-scheme" content="light"><meta name="supported-color-schemes" content="light">\n<title>' + esc(p.brand) + '</title>\n<link href="https://fonts.googleapis.com/css2?family=Geist:wght@400&display=swap" rel="stylesheet">\n<style>\n:root { color-scheme: light; supported-color-schemes: light; }\ntr.row td { transition: background-color 120ms ease-out; }\ntr.row:hover td { background-color: ' + C.hover + ' !important; }\ntr.row a { cursor: pointer; }\n@media (max-width: 480px) { td.page { padding-left: ' + C.s4 + " !important; padding-right: " + C.s4 + " !important; } table.builds > tbody > tr > th, table.builds > tbody > tr > td { padding-left: " + C.s1h + " !important; padding-right: " + C.s1h + ' !important; } table.builds td.mark { display: none !important; } }\n</style>\n</head>\n<body style="margin:0;padding:0;background:' + C.ground + ';-webkit-font-smoothing:antialiased;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" bgcolor="' + C.ground + '" style="background:' + C.ground + ';"><tr><td class="page" align="center" style="padding:' + C.s8 + " " + C.s6 + ';"><!--[if mso]><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" align="center"><tr><td><![endif]--><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="width:100%;max-width:600px;margin:0 auto;"><tr><td align="center" style="padding-bottom:' + C.s2 + ";font-family:" + C.face + ";font-size:" + C.title + ";font-weight:" + C.weight + ";line-height:" + C.titleLine + ";letter-spacing:" + C.tracking + ";color:" + C.ink + ';">' + esc(p.brand) + '</td></tr><tr><td align="center" style="padding-top:' + C.s4 + ";" + type2 + '">' + esc(p.lead) + "</td></tr>" + quote + card + '<tr><td align="center" style="padding-top:' + C.s4 + ';"><table role="presentation" cellpadding="0" cellspacing="0" border="0" align="center" style="margin:0 auto;"><tr><td bgcolor="' + C.brandFill + '" style="background:' + C.brandFill + ";border-radius:" + C.radiusBtn + ';"><a href="' + p.url + '" style="display:inline-block;padding:0 ' + C.s3 + ";height:" + C.btnHeight + ";line-height:" + C.btnHeight + ";font-family:" + C.face + ";font-size:" + C.body + ";font-weight:" + C.weight + ";color:" + C.onBrand + ";text-decoration:none;white-space:nowrap;border-radius:" + C.radiusBtn + ';">' + esc(p.cta) + '</a></td></tr></table></td></tr><tr><td align="center" style="padding-top:' + C.s4 + ";" + type2 + '">' + footer + '</td></tr></table><!--[if mso]></td></tr></table><![endif]--></td></tr></table></body>\n</html>';
};
const plain = (p: Page): string => {
  const out: string[] = p.lead.startsWith("Good ") ? [p.lead, ""] : ["Hi " + p.first + ",", "", p.lead, ""];
  for (const l of p.lines) out.push("  " + l.name + " (" + l.category + ") — data " + l.data + "/" + l.total + (l.shots >= 0 ? ", photos " + l.shots + "/" + l.total : "") + ", " + stateOf(l).due.toLowerCase());
  if (p.quote) out.push("", "  “" + p.quote + "”");
  out.push("", "  " + p.url, "", p.footer);
  return out.join("\n");
};
const greeting = (first: string, h: number): string => { const when = h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening"; return first === "" || first === "there" ? when + "," : when + " " + first + ","; };
const standing = (lines: L[]): string => { let open = 0, left = 0; for (const l of lines) { if (l.stage === "complete" || l.stage === "cancelled") continue; open++; left += Math.max(0, l.total - l.clean); } return "You have " + count(open, "build") + " open with " + count(left, "product") + " still to fill in."; };
const kind = text("kind");
const preview = I["preview"] === true;
const desk = (await archive.query("SELECT brand, url, support FROM intake LIMIT 1"))[0];
if (!desk) return refuse("the intake row is missing");
const brand = desk.brand as string, base = (desk.url as string).replace(/\/$/, ""), support = desk.support as string;
const hour = Number((await archive.query("SELECT extract(hour FROM now() AT TIME ZONE 'America/Los_Angeles')::int AS h"))[0]!.h);
const vendorFooter = "Stuck on a build? Reply to this email or write to " + support + ". Someone answers around the clock.";
const creatorFooter = "Reply to this email or write to " + support + ".";
const unsaved = (I["unsaved"] ?? null) as { vendor: string; name: string; template: string; category: string; mark: string; total: number; due: string; late: boolean; photos: boolean; emails: string[]; vendor_id: string } | null;
const builds = list("builds").length > 0 ? list("builds") : buildRef() !== "" ? [buildRef()] : [];
const head = builds[0] ? await archive.get("build", builds[0]) : null;
const vendorId = head ? head.vendor : unsaved ? unsaved.vendor_id : "";
const vendorRow = vendorId !== "" ? await archive.get("vendor", vendorId) : null;
const vendorName = vendorRow ? vendorRow.name : unsaved ? unsaved.vendor : "";
const contacts = (vendorRow ? vendorRow.contacts : []) as { name?: string; email?: string }[];
const hasLink = !!vendorRow && !!vendorRow.member;
let token = "link-created-when-sent";
if (!preview || hasLink) { if (vendorRow) { const t = await archive.run("vendor_token", { vendor: vendorRow.id }); if (t.error !== "") return { mail_error: "The vendor's link could not be made." }; token = (t.result as { token: string }).token; } }
const vendorUrl = base + "/v/" + token;
const rows = vendorId === "" ? [] : await archive.query("SELECT b.id, b.number, b.name, b.stage, b.wants_photos, b.skus, b.clean, b.data_clean, b.photos_clean, b.category_name, b.category, b.category_icon, coalesce(b.due_at < now(), false) AS late, coalesce(to_char(b.due_at AT TIME ZONE 'America/Los_Angeles', 'Mon FMDD'), '—') AS due, coalesce(to_char(b.submitted_at AT TIME ZONE 'America/Los_Angeles', 'Mon FMDD'), '—') AS submitted, coalesce(to_char(b.approved_at AT TIME ZONE 'America/Los_Angeles', 'Mon FMDD'), '—') AS approved FROM build_book b WHERE b.vendor_id = $1::uuid AND (b.stage NOT IN ('draft', 'cancelled') OR b.id = ANY($2::uuid[])) ORDER BY b.number", vendorId, builds);
const asLine = (r: Row): L => ({ id: r.id as string, number: r.number as number, name: r.name as string, stage: r.stage as string, photos: r.wants_photos as boolean, total: r.skus as number, clean: r.clean as number, data: r.data_clean as number, shots: r.wants_photos ? (r.photos_clean as number) : -1, category: r.category_name as string, mark: (r.category_icon as string) || (r.category as string), late: r.late as boolean, due: r.due as string, submitted: r.submitted as string, approved: r.approved as string, url: vendorUrl + "/" + r.number });
const rank = (l: L): number => l.stage === "review" ? 1 : l.stage === "complete" ? 2 : 0;
let lines = rows.map(r => asLine(r)).sort((a, b) => rank(a) - rank(b));
if (unsaved) lines = [...lines, { id: "", number: 0, name: unsaved.name, stage: "sent", photos: unsaved.photos, total: unsaved.total, clean: 0, data: 0, shots: unsaved.photos ? 0 : -1, category: unsaved.category, mark: unsaved.mark, late: unsaved.late, due: unsaved.due, submitted: "—", approved: "—", url: vendorUrl }];
const mine = lines.find(l => l.id === builds[0]);
const emails = list("to").length > 0 ? list("to") : head ? head.emails : unsaved ? unsaved.emails : [];
const firstName = (to: string[]): string => { const m = contacts.find(c => fold(c.email ?? "") === fold(to[0] ?? "")); const w = (m?.name ?? "").trim().split(/\s+/)[0]; return w ? w : "there"; };
const first = firstName(emails);
type M = { to: string[]; replyTo: string; subject: string; html: string; text: string };
const make = (to: string[], subject: string, p: Page): M => ({ to, replyTo: support, subject, html: shell(p, support), text: plain(p) });
const vendorPage = (lead: string, ls: L[], quote?: string): Page => ({ brand, first, lead, lines: ls, cta: "Open my builds", url: vendorUrl, footer: vendorFooter, ...(quote ? { quote } : {}) });
const creatorPage = (lead: string, ls: L[]): Page => ({ brand, first: (head?.created_by_name ?? "").trim().split(/\s+/)[0] || "there", lead, lines: ls, cta: "Open " + vendorName + "’s builds", url: vendorUrl, footer: creatorFooter });
const invite = (to: string[]): M => make(to, lines.length === 1 ? brand + " — " + lines[0]!.category + " build for " + vendorName : brand + " — " + count(lines.length, "build") + " for " + vendorName, vendorPage(greeting(first, hour) + " " + standing(lines), lines));
const reminder = (to: string[]): M => {
  const open = lines.filter(l => l.stage !== "complete" && l.stage !== "cancelled");
  const late = open.filter(l => l.late);
  const nudge = late.length === 0 ? "just a reminder of what is still open." : "just a reminder that " + (late.length === 1 ? "one of your builds is" : late.length + " of your builds are") + " past the date we asked for.";
  const subject = late.length === 0 ? "Still open — " + count(open.length, "build") + " for " + vendorName : "Overdue — " + count(late.length, "build") + " for " + vendorName;
  return make(to, subject, vendorPage(greeting(first, hour) + " " + nudge + " " + standing(open), open));
};
const send = async (m: M): Promise<string> => {
  if (m.to.length === 0) return "this build has no recipients yet; add an address before sending";
  let bad = "";
  for (const to of m.to) {
    try { await archive.email({ to, subject: m.subject, text: m.text, html: m.html, from: "intake" }); } catch (e) { bad = String(e); }
  }
  return bad;
};
const unlink = (m: M): M => { if (hasLink || !vendorRow && !unsaved) return m; const re = new RegExp(' href="' + vendorUrl.replace(/[.*+?^]/g, "\\$&") + '[^"]*"', "g"); return { ...m, html: m.html.replace(re, ' aria-disabled="true"').replaceAll(">Open my builds<", ">Open my builds (link created on send)<"), text: m.text.replaceAll(vendorUrl, "The link is created when this is sent.") }; };
const answer = async (m: M, what: string): Promise<Row> => { if (preview) return unlink(m) as never; const bad = await send(m); if (bad === "") return { sent: true }; console.error(bad); return { mail_error: "The " + what + " email could not be sent." }; };
const l1 = mine ? [mine] : [];
if (kind === "invite") { const r = await answer(invite(emails), "invite"); if (!preview && !r["mail_error"] && head && head.created_by.trim() !== "") { const s = mine ? count(mine.total, "SKU") : ""; await send(make([head.created_by], "“" + head.name + "” sent to " + vendorName, creatorPage("“" + head.name + "” went to " + vendorName + " at " + emails.join(", ") + " — " + s + " on it.", l1))); } return r; }
if (kind === "added") return answer(invite(emails), "invite");
if (kind === "reminder") return answer(reminder(emails), "reminder");
if (!head) return refuse("no such build");
if (kind === "returned") return answer(make(emails, "Sent back — " + head.name + " for " + vendorName, vendorPage(greeting(first, hour) + " " + brand + " sent “" + head.name + "” back to you with a note.", l1, head.return_note)), "return note");
if (kind === "withdrawn") return answer(make(emails, "Withdrawn — " + head.name + " for " + vendorName, vendorPage(greeting(first, hour) + " " + brand + " has withdrawn “" + head.name + "”. It drops off your page; nothing you entered is lost.", [])), "withdrawal");
if (kind === "approved") return answer(make(emails, "Approved — " + head.name + " for " + vendorName, vendorPage(greeting(first, hour) + " “" + head.name + "” is approved, thank you.", l1)), "approval");
if (kind === "review") { if (head.created_by.trim() === "") return { sent: false }; return answer(make([head.created_by], vendorName + " submitted “" + head.name + "”", creatorPage(vendorName + " submitted “" + head.name + "”: all " + count(head.skus, "SKU") + " complete, ready for review", l1)), "review"); }
return refuse("kind must be invite or reminder");
`;

const SEED = String.raw`
const seedRows = (spec: Spec, inputRows: unknown[]) => {
  const known = new Set(spec.fields.map(f => f.key));
  const seen = new Set<string>();
  const rows: { sku: string; name: string; values: Record<string, string> }[] = [];
  const repeats: string[] = [];
  for (const raw of inputRows) {
    const r = (typeof raw === "string" ? { sku: raw } : raw ?? {}) as { sku?: unknown; name?: unknown; values?: Record<string, unknown> };
    const vals = r.values ?? {};
    let sku = String(r.sku ?? "").trim() || String(vals["manufacturer_sku"] ?? "").trim();
    let name = String(r.name ?? "").trim() || String(vals["vendor_product_name"] ?? "").trim();
    if (!known.has("manufacturer_sku")) sku = "";
    if (!known.has("vendor_product_name")) name = "";
    const key = sku !== "" ? "sku\u0000" + fold(sku) : name !== "" ? "name\u0000" + fold(name) : "";
    if (key === "") continue;
    if (seen.has(key)) { repeats.push(sku || name); continue; }
    seen.add(key);
    const values: Record<string, string> = {};
    for (const [k, v] of Object.entries(vals)) { const t = String(v ?? "").trim(); if (known.has(k) && t !== "") values[k] = t; }
    if (sku !== "") values["manufacturer_sku"] = sku;
    if (name !== "") values["vendor_product_name"] = name;
    rows.push({ sku, name, values });
  }
  return { rows, repeats };
};
const recipients = (raw: string[]): string[] => {
  const out: string[] = [];
  for (const e of raw) { if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e)) refuse(JSON.stringify(e) + " is not an email address"); if (!out.some(h => fold(h) === fold(e))) out.push(e); }
  return out;
};
`;

const CELL = String.raw`
type Grid = { name: string; rows: string[][] };
const colNum = (ref: string): number => { const m = /^[A-Z]+/.exec(ref); let n = 0; for (const c of (m ? m[0] : "A")) n = n * 26 + c.charCodeAt(0) - 64; return n - 1; };
const rowNum = (ref: string): number => { const m = /\d+$/.exec(ref); return m ? Number(m[0]) - 1 : 0; };
const csvRows = (s: string): string[][] => {
  const d = (s.split("\n")[0] ?? "").includes("\t") ? "\t" : (s.split("\n")[0] ?? "").includes(";") && !(s.split("\n")[0] ?? "").includes(",") ? ";" : ",";
  const out: string[][] = []; let row: string[] = [], cur = "", q = false;
  for (let i = 0; i < s.length; i++) { const c = s[i] as string;
    if (q) { if (c === '"') { if (s[i + 1] === '"') { cur += '"'; i++; } else q = false; } else cur += c; continue; }
    if (c === '"') q = true; else if (c === d) { row.push(cur); cur = ""; } else if (c === "\n") { row.push(cur.replace(/\r$/, "")); out.push(row); row = []; cur = ""; } else cur += c; }
  if (cur !== "" || row.length > 0) { row.push(cur); out.push(row); }
  return out;
};
const readBook = async (id: string): Promise<{ name: string; sheets: Grid[] }> => {
  const f = await archive.file(id);
  if (/\.(xlsx|xlsm)$/i.test(f.name) || f.mime.includes("spreadsheetml")) {
    const m = await archive.read(id);
    if (m.type !== "xlsx") return refuse("Could not read " + f.name + ". Excel or CSV works best.");
    const sheets: Grid[] = m.sheets.map(s => { const rows: string[][] = []; for (const c of s.cells) { const r = rowNum(c.ref), k = colNum(c.ref); while (rows.length <= r) rows.push([]); const row = rows[r] as string[]; while (row.length <= k) row.push(""); row[k] = c.value ?? ""; } return { name: s.name, rows }; });
    return { name: f.name, sheets };
  }
  if (/\.(csv|tsv|txt)$/i.test(f.name) || f.mime.startsWith("text/")) return { name: f.name, sheets: [{ name: f.name, rows: csvRows(f.text) }] };
  return refuse("Could not read " + f.name + ". Excel or CSV works best.");
};
const guidance = (r: string[]): boolean => (r[0] ?? "").trim().startsWith("ⓘ");
const headerAt = (g: string[][]): number => { let at = 0, filled = 0; for (let i = 0; i < g.length && i < 15; i++) { if (guidance(g[i] as string[])) continue; const n = (g[i] as string[]).filter(c => c.trim() !== "").length; if (n > filled) { at = i; filled = n; } } return at; };
const pickSheet = (b: { sheets: Grid[] }): Grid => { const hit = b.sheets.find(s => /vendor|fill|build|product|sku/i.test(s.name)); if (hit) return hit; let best = b.sheets[0] as Grid, w = 0; for (const s of b.sheets) { const n = (s.rows[headerAt(s.rows)] ?? []).filter(c => c.trim() !== "").length; if (n > w && s.rows.length > headerAt(s.rows) + 1) { best = s; w = n; } } return best; };
const parseGrid = (g: string[][]): { headers: string[]; rows: Record<string, string>[] } => {
  if (g.length === 0) return { headers: [], rows: [] };
  const at = headerAt(g); const raw = (g[at] as string[]).map(c => c.trim()); const rows: Record<string, string>[] = [];
  for (const l of g.slice(at + 1)) { if (guidance(l)) continue; const r: Record<string, string> = {}; let some = false; raw.forEach((h, i) => { if (h === "") return; const v = (l[i] ?? "").trim(); if (v !== "") some = true; r[h] = v; }); if (some) rows.push(r); }
  return { headers: raw.filter(h => h !== ""), rows };
};
const exactMapping = (fields: Field[], headers: string[], synonyms: Record<string, string[]>): { mapping: Record<string, string>; unknown: string[] } => {
  const lookup: Record<string, string> = {}; const near: Record<string, string[]> = {}; const syn: Record<string, string[]> = {};
  for (const f of fields) lookup[squash(f.label)] = f.key;
  for (const f of fields) if (!lookup[squash(f.key)]) lookup[squash(f.key)] = f.key;
  const note = (into: Record<string, string[]>, run: string, key: string): void => { if (run === "") return; const l = into[run] ?? []; if (!l.includes(key)) l.push(key); into[run] = l; };
  for (const f of fields) { const w = f.label.trim().split(/\s+/); for (let i = 1; i < w.length; i++) note(near, squash(w.slice(i).join(" ")), f.key); const p = f.key.split("_"); for (let i = 1; i < p.length; i++) note(near, squash(p.slice(i).join(" ")), f.key); for (const s of synonyms[f.key] ?? []) note(syn, squash(s), f.key); }
  const mapping: Record<string, string> = {}; const unknown: string[] = [];
  for (const h of headers) { const k = squash(h); if (k === "") continue; const key = lookup[k] ?? (near[k]?.length === 1 ? near[k]![0] : undefined) ?? (syn[k]?.length === 1 ? syn[k]![0] : undefined); if (key === undefined || mapping[key]) { unknown.push(h); continue; } mapping[key] = h; }
  return { mapping, unknown };
};
const MODEL = "claude-haiku-5-5";
const askHeaders = async (spec: Spec, fields: Field[], left: string[], rows: Record<string, string>[], taken: Set<string>): Promise<Record<string, { field: string; sure: number }>> => {
  if (left.length === 0) return {};
  const options = fields.filter(f => !taken.has(f.key)).map(f => ({ key: f.key, label: f.label, options: f.options }));
  const columns = left.map(h => ({ heading: h, values: rows.map(r => (r[h] ?? "").trim()).filter(v => v !== "").slice(0, 5) }));
  const r = await archive.model({ model: MODEL, effort: "none", system: "Which field of the template is each column of the vendor's sheet? Answer only with a key you were given, or \"no field\". sure is how likely you are right, 0 to 1.", prompt: JSON.stringify({ template: spec.name, fields: options, columns }), schema: { type: "object", properties: { answers: { type: "array", items: { type: "object", properties: { heading: { type: "string" }, field: { type: "string" }, sure: { type: "number" } }, required: ["heading", "field", "sure"] } } }, required: ["answers"] } });
  const out: Record<string, { field: string; sure: number }> = {};
  for (const a of (r.json as { answers: { heading: string; field: string; sure: number }[] }).answers) out[a.heading] = { field: a.field, sure: a.sure };
  return out;
};
const optionFor = (f: Field, v: string): string => { const w = squash(v); return w === "" ? "" : (f.options ?? []).find(o => squash(o) === w) ?? ""; };
const multiParts = (v: string): string[] => v.split(/\s*[;\n]\s*/).map(p => p.trim()).filter(p => p !== "");
const spelledCell = (f: Field, v: string): string => {
  if (f.type === "yesno") { const l = v.trim().toLowerCase(); if (["y", "yes", "true", "1", "x", "✓"].includes(l)) return "Yes"; if (["n", "no", "false", "0"].includes(l)) return "No"; return v; }
  if (f.type === "select") { if (f.multi) { const ps = multiParts(v); const out: string[] = []; for (const p of ps) { const o = optionFor(f, p); if (o === "") return v; if (!out.includes(o)) out.push(o); } return out.length ? out.join("; ") : v; } return optionFor(f, v) || v; }
  return v;
};
const isSpelled = (f: Field, v: string): boolean => { if (f.type === "yesno") return v === "Yes" || v === "No"; if (f.type === "select") { const ps = f.multi ? multiParts(v) : [v]; return ps.every(p => (f.options ?? []).includes(p)); } return true; };
`;

const SPEC = String.raw`
const b = await loadBuild();
const spec = await specOf(b.template);
if (!caller.scope) return spec;
const fields = spec.fields.filter(f => f.owner === "vendor");
const keep = new Set(fields.map(f => f.key));
const columns = Object.fromEntries(Object.entries(spec.export.columns ?? {}).filter(([k]) => keep.has(k)));
const compose = Object.fromEntries(Object.entries(spec.export.compose ?? {}).filter(([k]) => keep.has(k)));
return { ...spec, shell: null, fields, export: { ...spec.export, columns, compose } };
`;

const BLANK = String.raw`
const b = await loadBuild();
const spec = await specOf(b.template);
const fields = spec.fields.filter(f => f.owner === "vendor");
const rows = await rowsOf(b.id);
const name = safeName(b.name) + "_template.xlsx";
const f = await archive.create((safeName(b.name) + "_template_" + Date.now().toString(36) + ".xlsx") as never);
const sheet = "Sheet1";
const col = (n: number): string => { let s = ""; for (let i = n + 1; i > 0; i = Math.floor((i - 1) / 26)) s = String.fromCharCode(65 + (i - 1) % 26) + s; return s; };
const guide = (x: Field): string => {
  const parts: string[] = [x.required ? "Required" : (x.requiredWhen ?? []).length > 0 ? "Required on a condition" : "Optional"];
  if (x.type === "select") parts.push((x.multi ? "One or more of: " : "One of: ") + (x.options ?? []).join(", "));
  if (x.type === "yesno") parts.push("Yes or No");
  if (x.type === "number") parts.push("A number, no units");
  if (x.type === "country") parts.push("Full country name");
  if (x.type === "dimensions") parts.push((x.parts ?? ["W", "D", "H"]).join(" x ") + " in inches");
  if (x.help) parts.push(x.help);
  return "ⓘ " + parts.join(". ");
};
const edits: Edit[] = [];
fields.forEach((x, i) => {
  edits.push({ op: "set_cell", cell: col(i) + "1", value: x.label + (x.required ? " *" : ""), sheet });
  edits.push({ op: "set_cell", cell: col(i) + "2", value: guide(x), sheet });
});
rows.forEach((r, n) => fields.forEach((x, i) => { const v = r.v[x.key]; if (v !== undefined && v !== "") edits.push({ op: "set_cell", cell: col(i) + (n + 3), value: v, sheet }); }));
const done = await archive.edit(made<{ id: string }>(f).id, edits);
if (caller.scope) await archive.update("file", done.id, { scope: caller.scope } as never);
return { file: done.id, name };
`;

const EXPORT = String.raw`
const ref = buildRef();
if (ref === "") refuse("id is required");
const asProduct = (await archive.query("SELECT build FROM build_row WHERE id::text = $1", ref))[0];
const buildId = asProduct ? String(asProduct.build) : ref;
const b = await (async () => { try { return await archive.get("build", buildId); } catch { return refuse("no such build"); } })();
const spec = await specOf(b.template);
const all = await rowsOf(b.id);
const rows = asProduct ? all.filter(r => r.id === ref) : all;
const safe = asProduct ? safeName(listedAs(rows[0]!.sku, rows[0]!.name)) : safeName(b.name);
const resume = text("folder");
const at = typeof I["at"] === "number" ? I["at"] as number : 0;
const started = Date.now();
const folder = resume !== "" ? { id: resume } : made<{ id: string }>(await archive.create("file", { kind: "folder", name: safe + " " + Date.now().toString(36) } as never));
const col = (n: number): string => { let s = ""; for (let i = n + 1; i > 0; i = Math.floor((i - 1) / 26)) s = String.fromCharCode(65 + (i - 1) % 26) + s; return s; };
const exportValue = (f: Field, v: string | undefined): string => {
  if (v === undefined || v.trim() === "" || v.trim().toLowerCase() === "n/a") return "";
  const c = spec.export.compose?.[f.key];
  if (!c || !c.parts || c.parts.length === 0) return v;
  const one = (cell: string): string => { const nums = cell.match(/\d+(\.\d+)?/g) ?? []; if (nums.length !== c.parts!.length) return cell; return nums.map((n, i) => (c.parts![i]!.prefix ?? "") + n + (c.parts![i]!.suffix ?? "")).join(c.join || " x "); };
  return f.multi ? v.split(/\s*[;\n]\s*/).filter(p => p.trim() !== "").map(one).join("; ") : one(v);
};
const edits: Edit[] = [];
let workbook = "";
if (resume !== "") {
} else if (spec.shell) {
  const copy = await archive.copy(spec.shell, { name: safe + ".xlsx" });
  const model = await archive.read(copy.id);
  if (model.type !== "xlsx") return refuse("The template's workbook could not be read.");
  const sheet = model.sheets.find(s => s.name === spec.sheet) ?? model.sheets[0];
  if (!sheet) return refuse("The template's workbook could not be read.");
  const first = (spec.export.row ?? 0) >= 2 ? spec.export.row as number : 2;
  const labels = new Map<string, string>();
  for (const c of sheet.cells) { const r = /\d+$/.exec(c.ref); if (r && Number(r[0]) < first && c.value) { const k = squash(c.value); if (k !== "" && !labels.has(k)) labels.set(k, c.ref.replace(/\d+$/, "")); } }
  rows.forEach((r, n) => { for (const f of spec.fields) { const letter = (spec.export.columns?.[f.key] ?? "").trim().toUpperCase() || labels.get(squash(f.label)); if (!letter) continue; const v = exportValue(f, r.v[f.key]); if (v !== "") edits.push({ op: "set_cell", cell: letter + (first + n), value: v, sheet: sheet.name }); } });
  workbook = (await archive.edit(copy.id, edits)).id;
} else {
  const blank = made<{ id: string }>(await archive.create((safe + ".xlsx") as never));
  spec.fields.forEach((f, i) => edits.push({ op: "set_cell", cell: col(i) + "1", value: f.label }));
  rows.forEach((r, n) => spec.fields.forEach((f, i) => { const v = exportValue(f, r.v[f.key]); if (v !== "") edits.push({ op: "set_cell", cell: col(i) + (n + 2), value: v }); }));
  workbook = (await archive.edit(blank.id, edits)).id;
}
if (resume === "") await archive.update("file", workbook, { parent: folder.id });
if (b.photos) {
  const images = asProduct ? await archive.query("SELECT i.id, i.name, i.file, i.role, i.build_row FROM build_image i WHERE i.build_row = $1::uuid ORDER BY i.position", ref) : await archive.query("SELECT i.id, i.name, i.file, i.role, i.build_row FROM build_image i WHERE i.build = $1::uuid ORDER BY i.position", b.id);
  const listed = new Map(all.map(r => [r.id, listedAs(r.sku, r.name)]));
  const seg = (s: string): string => { const t = s.replace(/[/\\]/g, "-").trim(); return t === "" || t === "." || t === ".." ? "_" : t; };
  const folders = new Map<string, string>();
  const dir = async (name: string, parent: string): Promise<string> => { const k = parent + "/" + name; const had = folders.get(k); if (had) return had; const there = await archive.query("SELECT id FROM archive.file WHERE parent = $1::uuid AND name = $2 AND kind = 'folder'", parent, name); if (there[0]) { folders.set(k, String(there[0].id)); return String(there[0].id); } const d = made<{ id: string }>(await archive.create("file", { kind: "folder", name, parent } as never)); folders.set(k, d.id); return d.id; };
  const imagesDir = images.length ? await dir("images", folder.id) : "";
  const filed = new Map<string, number>();
  if (resume === "") {
  const tab = made<{ id: string }>(await archive.create("IMAGES.xlsx" as never));
  const tabEdits: Edit[] = [{ op: "set_cell", cell: "A1", value: "SKU" }, { op: "set_cell", cell: "B1", value: "Images" }];
  rows.forEach((r, n) => { tabEdits.push({ op: "set_cell", cell: "A" + (n + 2), value: listedAs(r.sku, r.name) }); images.filter(i => i.build_row === r.id).forEach((i, k) => tabEdits.push({ op: "set_cell", cell: col(k + 1) + (n + 2), value: (String(i.role) || "?") + ": " + String(i.name) })); });
  await archive.edit(tab.id, tabEdits);
  await archive.update("file", tab.id, { parent: folder.id });
  }
  let n0 = -1;
  for (const i of images) {
    n0++;
    const name = String(i.name);
    const m = /\.[^.]+$/.exec(name);
    const ext = m ? m[0].toLowerCase() : "";
    const product = listed.get(String(i.build_row)) ?? "";
    const placed = product !== "" && String(i.role) !== "";
    const base = placed ? seg(product) : seg(name.slice(0, name.length - ext.length) || "photo");
    const parent = await dir(placed ? seg(String(i.role)) : "unplaced", imagesDir);
    const key = parent + "/" + base;
    const n = (filed.get(key) ?? 0) + 1;
    filed.set(key, n);
    if (n0 < at) continue;
    if (Date.now() - started > 20000) return { url: "/api/file/" + folder.id + "/zip", filename: safe + ".zip", folder: folder.id, done: false, at: n0, total: images.length };
    const c = await archive.copy(String(i.file), { name: base + (n > 1 ? "_" + n : "") + ext });
    await archive.update("file", c.id, { parent });
  }
}
return { url: "/api/file/" + folder.id + "/zip", filename: safe + ".zip", folder: folder.id, done: true };
`;

const WF = {
  mail: { runs: null, source: PRE + 'const ICONS_TEXT = ' + JSON.stringify(JSON.stringify(ICONS_DATA)) + ';\n' + MAIL },
  vendor_token: { runs: null, source: PRE + String.raw`
const v = await archive.get("vendor", text("vendor"));
const rotate = I["rotate"] === true;
let member = v.member;
if (!member) {
  if (I["mint"] === false) return { token: "", member: "" };
  const g = await archive.query('SELECT id FROM archive."group" WHERE name = $1', "vendors");
  const m = await archive.create("member", { role: "guest", scope: v.id, name: v.name, group: g[0]!.id } as never);
  member = (m as { id: string }).id;
  await archive.update("vendor", v.id, { member, link_at: now() });
}
const token = await archive.link(member, { rotate });
if (rotate) await archive.update("vendor", v.id, { link_at: now() });
return { token, member };
` },
  vendor_link: { runs: 'buyers', source: PRE + String.raw`
const v = await archive.get("vendor", text("vendor"));
if (!v.member) return { vendor: v.id, token: "", url: "", minted_at: v.link_at };
const t = await archive.run("vendor_token", { vendor: v.id });
if (t.error !== "") return refuse(t.error);
const d = await archive.query("SELECT url FROM intake LIMIT 1");
const token = (t.result as { token: string }).token;
return { vendor: v.id, token, url: String(d[0]!.url).replace(/\/$/, "") + "/v/" + token, minted_at: v.link_at };
` },
  rotate_vendor_link: { runs: 'buyers', source: PRE + String.raw`
const v = await archive.get("vendor", text("vendor"));
const t = await archive.run("vendor_token", { vendor: v.id, rotate: !!v.member });
if (t.error !== "") return refuse(t.error);
const d = await archive.query("SELECT url FROM intake LIMIT 1");
const token = (t.result as { token: string }).token;
return { vendor: v.id, token, url: String(d[0]!.url).replace(/\/$/, "") + "/v/" + token, rotated: !!v.member };
` },
  create_build: { runs: 'buyers', source: PRE + SEED + String.raw`
const name = text("name");
if (name === "") refuse("Give the build a name. The vendor sees it too.");
const ref = text("template");
if (ref === "") refuse("template is required");
const found = await archive.query("SELECT id, name, key FROM template WHERE id::text = $1 OR key = $1", ref);
const leaf = found[0];
if (!leaf) return refuse("no such template " + JSON.stringify(ref));
const spec = await specOf(leaf.id as string);
const vRef = text("vendor");
if (vRef === "") refuse("Which vendor is this for?");
const byId = await archive.query("SELECT id, name, contacts FROM vendor WHERE id::text = $1 OR lower(name) = lower($1)", vRef);
const vendor = byId[0] ?? made<Row>(await archive.create("vendor", { name: vRef } as never));
const vendorId = vendor.id as string;
let emails = recipients(list("emails"));
if (emails.length === 0) {
  const saved = ((vendor.contacts ?? []) as { email?: string }[]).map(c => (c.email ?? "").trim()).filter(e => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e));
  emails = recipients(saved);
  if (emails.length === 0) {
    const last = await archive.query("SELECT emails FROM build WHERE vendor = $1::uuid AND cardinality(emails) > 0 ORDER BY (template::text = $2) DESC, number DESC LIMIT 1", vendorId, leaf.id as string);
    if (last[0]) emails = (last[0].emails ?? []) as string[];
  }
}
const send = I["send"] === true;
if (send && emails.length === 0) refuse(String(vendor.name) + " has no contact on file yet. Add one on the vendor before sending.");
const photos = I["photos"] !== false;
const kind = text("build_type") || "Dropship";
if (!["Owned", "Dropship", "MTO"].includes(kind)) refuse("A build is Owned, Dropship or MTO, not " + JSON.stringify(kind) + ".");
const inRows = Array.isArray(I["rows"]) ? I["rows"] as unknown[] : [];
const { rows, repeats } = seedRows(spec, inRows);
if (rows.length === 0) refuse("Add at least one product by its SKU or its name — paste them or drop the vendor’s sheet.");
const due = text("due");
const dueRows = due === "" ? await archive.query("SELECT ((now() AT TIME ZONE 'America/Los_Angeles')::date + 7 + time '12:00') AT TIME ZONE 'America/Los_Angeles' AS at") : /^\d{4}-\d{2}-\d{2}$/.test(due) ? await archive.query("SELECT ($1::date + time '12:00') AT TIME ZONE 'America/Los_Angeles' AS at", due) : await archive.query("SELECT $1::timestamptz AS at", due);
let every = typeof I["remind_every_days"] === "number" ? I["remind_every_days"] as number : 0;
if (every === 0) every = String(leaf.key).startsWith("made_to_order") ? 7 : 3;
if (every < 1) refuse("remind_every_days must be at least 1");
const source = text("source");
const creator = caller.name.trim() || caller.email;
const b = made<{ id: string; number: number }>(await archive.create("build", { name, created_by: caller.email, created_by_name: creator, vendor: vendorId, emails, template: leaf.id, stage: "draft", due_at: dueRows[0]!.at, remind_every_days: every, source, mapping: I["mapping"] ?? {}, columns: Array.isArray(I["columns"]) ? I["columns"] : [], column_map: I["column_map"] ?? {}, photos, build_type: kind } as never));
await archive.update("build", b.id, { seed: rows.map((r, position) => ({ position, name: r.name, sku: r.sku, ...r.values })) });
await line(b.id, "ba", "Build created by " + creator + " from the " + String(leaf.name) + " template, " + count(rows.length, "product") + (source !== "" ? " prefilled from " + source : ""));
const out: Row = { id: b.id, number: b.number, duplicates: repeats, elsewhere: await elsewhere(rows.filter(r => r.sku !== "").map(r => r.sku), rows.filter(r => r.sku === "").map(r => r.name), b.id) };
if (send) {
  const at = now();
  await archive.update("build", b.id, { stage: "sent", sent_at: at });
  const bad = await mail("invite", { build: b.id });
  if (bad !== "") out["invite_error"] = "The invite email could not be sent. Share the link directly.";
  await line(b.id, "system", bad === "" ? "Link emailed to " + emails.join(", ") : "The link could not be emailed to " + emails.join(", ") + "; share it directly");
}
return out;
` },
  send_build: { runs: 'buyers', source: PRE + String.raw`
const b = await loadBuild();
if (b.stage !== "draft") refuse("Only a draft can be sent to the vendor; this build is " + words(b.stage) + ".");
if (b.emails.length === 0) refuse((await archive.get("vendor", b.vendor)).name + " has no contact on file yet. Add one on the vendor before sending.");
await archive.update("build", b.id, { stage: "sent", sent_at: now() });
const bad = await mail("invite", { build: b.id });
const to = b.emails.join(", ");
await line(b.id, "system", bad === "" ? "Link emailed to " + to : "The link could not be emailed to " + to + "; share it directly");
return bad === "" ? { id: b.id, stage: "sent" } : { id: b.id, stage: "sent", invite_error: "The invite email could not be sent. Share the link directly." };
` },
  remind_build: { runs: 'buyers', source: PRE + String.raw`
const b = await loadBuild();
if (b.stage !== "sent" && b.stage !== "in_progress") refuse("A reminder goes to a build that is out with the vendor; this one is " + words(b.stage) + ".");
const left = b.skus - b.clean;
const bad = await mail("reminder", { build: b.id });
if (bad === "") await archive.update("build", b.id, { reminders_sent: b.reminders_sent + 1, last_reminded_at: now() });
await line(b.id, "system", bad === "" ? "Reminder sent by you: " + left + " SKU" + plural(left) + " still need attention" : "The reminder could not be emailed to " + b.emails.join(", "));
return bad === "" ? { id: b.id, reminders_sent: b.reminders_sent + 1 } : { id: b.id, reminders_sent: b.reminders_sent, mail_error: "The reminder email could not be sent." };
` },
  remind: { runs: null, source: PRE + String.raw`
const due = await archive.query("SELECT b.id, b.vendor, b.emails, b.skus, b.clean, b.reminders_sent, b.sent_at, b.remind_every_days, floor(extract(epoch FROM now() - b.sent_at) / (greatest(1, b.remind_every_days) * 86400))::int AS earned FROM build b WHERE b.stage IN ('sent', 'in_progress') AND b.sent_at IS NOT NULL AND floor(extract(epoch FROM now() - b.sent_at) / (greatest(1, b.remind_every_days) * 86400))::int > b.reminders_sent AND ($1::text[] IS NULL OR b.id::text = ANY($1::text[]))", list("builds").length > 0 ? list("builds") : null);
const groups = new Map<string, typeof due>();
for (const b of due) { const key = String(b.vendor) + "|" + ((b.emails ?? []) as string[]).map(fold).sort().join(","); groups.set(key, [...(groups.get(key) ?? []), b]); }
let failed = 0, sent = 0;
for (const g of groups.values()) {
  const bad = await mail("reminder", { builds: g.map(b => String(b.id)) });
  if (bad !== "") { failed++; continue; }
  sent++;
  for (const b of g) {
    const left = Number(b.skus) - Number(b.clean);
    const to = ((b.emails ?? []) as string[]).join(", ");
    for (let k = Number(b.reminders_sent) + 1; k <= Number(b.earned); k++) await line(String(b.id), "system", "Reminder " + k + " sent to " + to + ": " + left + " " + (left === 1 ? "SKU" : "SKUs") + " still need attention");
    await archive.update("build", String(b.id), { reminders_sent: Number(b.earned), last_reminded_at: now() });
  }
}
if (failed > 0) throw new Error(failed + " reminder(s) failed");
return { due: due.length, sent };
` },
  return_build: { runs: 'buyers', source: PRE + String.raw`
const note = text("note");
if (note === "") refuse("Say what needs to change; the note is what the vendor sees.");
const b = await loadBuild();
if (b.stage !== "review") refuse("Only a build in review can be sent back; this one is " + words(b.stage) + ".");
await archive.update("build", b.id, { stage: "in_progress", return_note: note, returned_at: now(), submitted_at: null });
const bad = await mail("returned", { build: b.id });
const to = b.emails.join(", ");
await line(b.id, "ba", "Sent back to vendor: “" + note + "” — " + (bad === "" ? "emailed to " + to : "the note could not be emailed to " + to));
return bad === "" ? { id: b.id, stage: "in_progress" } : { id: b.id, stage: "in_progress", mail_error: "The return note email could not be sent." };
` },
  approve_build: { runs: 'buyers', source: PRE + String.raw`
const b = await loadBuild();
if (b.stage !== "sent" && b.stage !== "in_progress" && b.stage !== "review") refuse("Only a build out with the vendor or in review can be approved; this one is " + words(b.stage) + ".");
const file = safeName(b.name) + ".zip";
await archive.update("build", b.id, { stage: "complete", approved_at: now() });
const bad = await mail("approved", { build: b.id });
const to = b.emails.join(", ");
await line(b.id, "ba", "Approved" + (b.errors > 0 ? " with " + count(b.errors, "open issue") : "") + " and exported " + file + " — handed to merch; " + (bad === "" ? "the thank-you was emailed to " + to : "the thank-you could not be emailed to " + to));
return bad === "" ? { id: b.id, stage: "complete", file } : { id: b.id, stage: "complete", file, mail_error: "The approval email could not be sent." };
` },
  cancel_build: { runs: 'buyers', source: PRE + String.raw`
const b = await loadBuild();
if (b.stage === "complete") refuse("This build is already approved; there is nothing to withdraw — merch has the deliverable and its products are captured. Delete it if it should not have been approved; the products it captured stay.");
if (b.stage === "cancelled") refuse("This build is already cancelled.");
await archive.update("build", b.id, { stage: "cancelled", cancelled_at: now() });
let t = "Cancelled";
let bad = "";
if (b.sent_at) { bad = await mail("withdrawn", { build: b.id }); const to = b.emails.join(", "); t += " — " + (bad === "" ? to + " told the request is withdrawn" : "the withdrawal could not be emailed to " + to); }
await line(b.id, "ba", t);
return bad === "" ? { id: b.id, stage: "cancelled" } : { id: b.id, stage: "cancelled", mail_error: "The withdrawal email could not be sent." };
` },
  delete_build: { runs: 'buyers', source: PRE + String.raw`
const b = await loadBuild();
if (b.stage !== "draft" && b.stage !== "cancelled" && b.stage !== "complete") refuse("This build is " + words(b.stage) + ". Cancel it first so the vendor is told, then delete it.");
const shots = await archive.query("SELECT i.file FROM build_image i WHERE i.build = $1::uuid", b.id);
await archive.delete("build", b.id);
let kept = 0;
for (const s of shots) { try { await archive.delete("file", String(s.file)); } catch { kept++; } }
return { id: b.id, deleted: true, ...(kept > 0 ? { files_kept: kept } : {}) };
` },
  set_build_emails: { runs: 'buyers', source: PRE + SEED + String.raw`
const b = await loadBuild();
if (b.stage === "complete" || b.stage === "cancelled") refuse("This build is " + words(b.stage) + "; its recipients are final.");
const emails = recipients(list("emails"));
const out = b.stage !== "draft";
if (out && emails.length === 0) refuse("A build that is out needs at least one recipient.");
const added = emails.filter(e => !b.emails.some(h => fold(h) === fold(e)));
await archive.update("build", b.id, { emails });
const answer: Row = { id: b.id, emails, added };
if (!out || added.length === 0) { await line(b.id, "ba", "Recipients set to " + (emails.length ? emails.join(", ") : "nobody yet")); return answer; }
const bad = await mail("added", { build: b.id, to: added });
if (bad !== "") answer["mail_error"] = "The invite email could not be sent.";
await line(b.id, "system", bad === "" ? "Link emailed to " + added.join(", ") + ", added to the build" : added.join(", ") + " added to the build; the link could not be emailed to them");
return answer;
` },
  set_build_photos: { runs: 'buyers', source: PRE + String.raw`
const b = await loadBuild();
if (b.stage === "complete" || b.stage === "cancelled") refuse("This build is " + words(b.stage) + "; what it asks for is final.");
const photos = I["photos"] === true;
if (b.photos === photos) return { id: b.id, photos };
const after = await archive.update("build", b.id, { photos });
const fresh = await archive.get("build", b.id);
await line(b.id, "ba", photos ? "Photographs asked for" : "Photographs no longer asked for");
return { id: after.id, photos, errors: fresh.errors, clean: fresh.clean };
` },
  edit_build_skus: { runs: 'buyers', source: PRE + SEED + String.raw`
const b = await loadBuild();
if (b.stage === "complete" || b.stage === "cancelled") refuse("This build is " + words(b.stage) + " and takes no more changes.");
const spec = await specOf(b.template);
const remove = list("remove");
const drop = new Set(remove.map(fold));
const addIn = Array.isArray(I["add"]) ? I["add"] as unknown[] : [];
const { rows: fresh } = seedRows(spec, addIn);
for (const r of fresh) if (drop.has(fold(r.sku)) || (r.sku === "" && drop.has(fold(r.name)))) refuse(listedAs(r.sku, r.name) + " is on both lists — add it or drop it, not both.");
if (fresh.length === 0 && drop.size === 0) refuse("Name at least one product to add or to drop.");
const stored = await rowsOf(b.id);
const gone = new Set<string>();
for (const named of remove) {
  const exact = stored.some(r => r.sku === named);
  for (const r of stored) if (r.id === named || (exact && r.sku === named) || (!exact && r.sku !== "" && fold(r.sku) === fold(named)) || (r.sku === "" && fold(r.name) === fold(named))) gone.add(r.id);
}
const have = new Set(stored.filter(r => !gone.has(r.id)).map(r => r.sku !== "" ? "s" + fold(r.sku) : "n" + fold(r.name)));
const add = fresh.filter(r => { const k = r.sku !== "" ? "s" + fold(r.sku) : "n" + fold(r.name); if (have.has(k)) return false; have.add(k); return true; });
const removed = stored.filter(r => gone.has(r.id)).map(r => listedAs(r.sku, r.name));
const added = add.map(r => listedAs(r.sku, r.name));
if (added.length === 0 && removed.length === 0) refuse("Nothing changed: the products to add are already on the build and the ones to drop are not.");
if (stored.length - gone.size + add.length === 0) refuse("That would drop every product. Withdraw the build instead.");
const files = await archive.query("SELECT i.file FROM build_image i WHERE i.build_row = ANY($1::uuid[])", [...gone]);
for (const id of gone) await archive.delete("build_row", id);
let at = stored.reduce((m, r) => Math.max(m, r.position), -1);
if (add.length > 0) await archive.update("build", b.id, { seed: add.map(r => { at++; return { position: at, name: r.name, sku: r.sku, ...r.values }; }) });
const few = (l: string[]): string => l.length <= 5 ? l.join(", ") : count(l.length, "product");
await line(b.id, who(), "Products changed — " + [added.length ? "added " + few(added) : "", removed.length ? "dropped " + few(removed) : ""].filter(x => x !== "").join(", "));
for (const f of files) { try { await archive.delete("file", String(f.file)); } catch { continue; } }
const after = await archive.get("build", b.id);
return { id: b.id, added, removed, skus: after.skus, stage: after.stage, elsewhere: await elsewhere(add.filter(r => r.sku !== "").map(r => r.sku), add.filter(r => r.sku === "").map(r => r.name), b.id) };
` },
  skus_elsewhere: { runs: 'buyers', source: PRE + String.raw`
return { elsewhere: await elsewhere(list("skus"), list("names"), text("build")) };
` },
  preview_mail: { runs: 'buyers', source: PRE + String.raw`
const kind = text("kind");
if (kind !== "invite" && kind !== "reminder") refuse("kind must be invite or reminder");
const saved = text("build");
if (saved !== "") { const r = await archive.run("mail", { kind, build: saved, preview: true }); if (r.error !== "") return refuse(r.error); return r.result; }
const name = text("name");
if (name === "") refuse("the build needs a name");
const inRows = Array.isArray(I["rows"]) ? I["rows"] as unknown[] : [];
if (inRows.length === 0) refuse("the build needs at least one product");
const t = (await archive.query("SELECT id, name, key, icon FROM template WHERE id::text = $1 OR key = $1", text("template")))[0];
if (!t) return refuse("no such template: " + text("template"));
const vRef = text("vendor");
if (vRef === "") refuse("the build needs a vendor");
const v = (await archive.query("SELECT id, name FROM vendor WHERE id::text = $1 OR lower(name) = lower($1)", vRef))[0];
const due = text("due");
const d = (await archive.query("SELECT coalesce(to_char($1::date, 'Mon FMDD'), '—') AS due, coalesce($1::date < now(), false) AS late", due === "" ? null : due.slice(0, 10)))[0]!;
const r = await archive.run("mail", { kind, preview: true, unsaved: { vendor: v ? v.name : vRef, vendor_id: v ? v.id : "", name, template: t.id, category: t.name, mark: String(t.icon) || String(t.key), total: inRows.length, due: d.due, late: d.late, photos: true, emails: list("emails") } });
if (r.error !== "") return refuse(r.error);
return r.result;
` },
  open_build: { runs: 'vendors', source: PRE + String.raw`
const b = await loadBuild();
if (!caller.scope || b.opened_at) return { id: b.id, opened: !!b.opened_at };
await archive.update("build", b.id, { opened_at: now() });
await line(b.id, "vendor", "Opened the link");
return { id: b.id, opened: true };
` },
  submit_build: { runs: 'vendors', source: PRE + String.raw`
const b = await loadBuild();
if (b.stage === "review") refuse("This build is already submitted; " + String((await archive.query("SELECT brand FROM intake LIMIT 1"))[0]!.brand) + " have it.");
if (b.stage !== "in_progress") refuse("Only a build being filled in can be submitted; this one is " + words(b.stage) + ".");
const left = b.skus - b.clean;
if (left > 0 || b.skus === 0) refuse(count(left, "product") + " " + (left === 1 ? "still needs" : "still need") + " something before this can be submitted.");
const brand = String((await archive.query("SELECT brand FROM intake LIMIT 1"))[0]!.brand);
await archive.update("build", b.id, { stage: "review", submitted_at: now(), return_note: "" });
await line(b.id, who(), "Submitted: " + b.skus + " of " + b.skus + " SKUs — sent to " + brand);
return { id: b.id, stage: "review" };
` },
  dismiss_note: { runs: 'vendors', source: PRE + String.raw`
const b = await loadBuild();
await archive.update("build", b.id, { return_note: "" });
return { id: b.id };
` },
  reset_build: { runs: 'vendors', source: PRE + String.raw`
const b = await openOf();
const shots = await archive.query("SELECT i.id, i.file FROM build_image i WHERE i.build = $1::uuid", b.id);
for (const s of shots) await archive.delete("build_image", String(s.id));
await archive.update("build", b.id, { reset_at: now(), sorting: 0 });
const n = shots.length;
await line(b.id, "vendor", "Reset by the vendor: the sheet is back to what was sent" + (n > 0 ? ", " + count(n, "photo") + " removed" : ""));
let kept = 0;
for (const s of shots) { try { await archive.delete("file", String(s.file)); } catch { kept++; } }
return { id: b.id, removed: n, ...(kept > 0 ? { files_kept: kept } : {}) };
` },
  attach_photos: { runs: 'vendors', source: PRE + String.raw`
const b = await openOf();
if (!b.photos) refuse("This build does not ask for photographs.");
const photos = (Array.isArray(I["photos"]) ? I["photos"] : []) as { file?: string; name?: string }[];
if (photos.length === 0) refuse("No photo was attached.");
const MEDIA: Record<string, string> = { "image/jpeg": "jpeg", "image/png": "png", "image/tiff": "tiff", "image/gif": "gif", "image/webp": "webp", "image/bmp": "bmp", "image/heic": "heic", "image/avif": "avif", "video/mp4": "mp4", "video/quicktime": "mov", "video/webm": "webm" };
const skipped: string[] = [];
let position = Number((await archive.query("SELECT coalesce(max(position), -1) + 1 AS n FROM build_image WHERE build = $1::uuid", b.id))[0]!.n);
let landed = 0;
for (const p of photos) {
  const id = String(p.file ?? "").trim();
  if (!/^[0-9a-f-]{36}$/i.test(id)) refuse("Each photo names its uploaded file.");
  const f = await archive.file(id);
  const name = String(p.name ?? "").trim() || f.name;
  const media = MEDIA[f.mime.toLowerCase()] ?? "";
  if (media === "") { skipped.push(name); continue; }
  const meta = f.metadata as { width?: number; height?: number };
  await archive.create("build_image", { build: b.id, file: id, name, position, media, width: meta.width ?? 0, height: meta.height ?? 0, transparent: media === "png", fill: 0, measured: true } as never);
  position++;
  landed++;
}
if (landed === 0) refuse((skipped[0] ?? "That") + " is not a photograph.");
await line(b.id, "vendor", "Uploaded " + count(landed, "photo"));
const placed = await archive.run("sort_photos_inner", { build: b.id });
const matched = placed.error === "" ? (placed.result as { matched: number }).matched : 0;
return { uploaded: landed, matched, ...(skipped.length ? { skipped } : {}), ...(placed.error !== "" ? { notice: placed.error.split("\n")[0] } : {}) };
` },
  sort_photos: { runs: 'vendors', source: PRE + String.raw`
const b = await openOf();
if (!b.photos) refuse("This build does not ask for photographs.");
const r = await archive.run("sort_photos_inner", { build: b.id });
if (r.error !== "") return refuse(r.error.split("\n")[0] ?? "");
return { uploaded: 0, matched: (r.result as { matched: number }).matched };
` },
  sort_photos_inner: { runs: null, source: PRE + String.raw`
const b = await loadBuild();
const spec = await specOf(b.template);
const pool = await archive.query("SELECT id, name, media FROM build_image WHERE build = $1::uuid AND build_row IS NULL ORDER BY position", b.id);
if (pool.length === 0) return { matched: 0 };
const rows = await rowsOf(b.id);
const labels = rows.map(r => listedAs(r.sku, r.name)).filter(l => l !== "");
const rowOf = new Map(rows.map(r => [listedAs(r.sku, r.name), r]));
const angles = (await archive.query("SELECT jsonb_agg(a) AS a FROM jsonb_array_elements(angles()) a"))[0]!.a as { key: string; hint: string; many?: boolean }[];
const vendor = await archive.get("vendor", b.vendor);
const mapped = vendor.photo_map as Record<string, string>;
const mappedAngle = (name: string): string => { const low = name.toLowerCase(); let best = ""; for (const p of Object.keys(mapped)) if (p.length > best.length && low.includes(p)) best = p; return mapped[best] ?? ""; };
const exact = (name: string): string => { let found = ""; for (const s of labels) { const q = s.replace(/[.*+?^()\[\]{}|\\$]/g, "\\$&"); if (new RegExp("(^|[^a-z0-9])" + q + "($|[^a-z0-9])", "i").test(name)) { if (found !== "") return ""; found = s; } } return found; };
const size = Math.max(1, Math.min(20, Math.floor(65536 / Math.max(1, 21 * labels.length))));
type Read = { id: string; name: string; media: string; sku: string; role: string; p: number };
const reads: Read[] = [];
const reqs: { names: string[]; at: number }[] = [];
for (let k = 0; k < pool.length; k += size) reqs.push({ names: pool.slice(k, k + size).map(p => String(p.name)), at: k });
const answers = reqs.length === 0 ? [] : await archive.model(reqs.map(q => ({ model: "claude-haiku-5-5", effort: "none" as const, system: "For each file name of a product photo answer two things: sku, which of the given products the name refers to (exactly one of them, or empty when it spells none), with sku_p how likely, 0 to 1; and angle, which kind of product photo it is by its name (exactly one of the given angle keys), with angle_p how likely.", prompt: JSON.stringify({ products: labels, angles: angles.map(a => ({ key: a.key, what: a.hint })), files: q.names }), schema: { type: "object", properties: { answers: { type: "array", items: { type: "object", properties: { file: { type: "string" }, sku: { type: "string" }, sku_p: { type: "number" }, angle: { type: "string" }, angle_p: { type: "number" } }, required: ["file", "sku", "sku_p", "angle", "angle_p"] } } }, required: ["answers"] } })));
reqs.forEach((q, i) => {
  const said = new Map(((answers[i]?.json ?? { answers: [] }) as { answers: { file: string; sku: string; sku_p: number; angle: string; angle_p: number }[] }).answers.map(a => [a.file, a]));
  q.names.forEach((name, j) => {
    const p = pool[q.at + j]!;
    const a = said.get(name);
    let sku = exact(name), sure = 1;
    if (sku === "") { if (!a || a.sku_p < 0.5 || !labels.includes(a.sku)) { sku = ""; } else { sku = a.sku; sure = a.sku_p; } }
    let role = mappedAngle(name), likely = 1;
    if (role === "" && a) { role = a.angle; likely = a.angle_p; }
    const media = String(p.media);
    if (["mp4", "mov", "webm"].includes(media)) role = "video"; else if (role === "video" && media !== "gif") role = "";
    if (!angles.some(x => x.key === role)) { sku = ""; role = ""; }
    reads.push({ id: String(p.id), name, media, sku, role, p: sure * likely });
  });
});
const slots = new Map<string, Record<string, number>>();
for (const r of rows) { const owed = (await archive.query("SELECT images_owed($1::jsonb, $2::jsonb) AS o", JSON.stringify(spec), JSON.stringify(r.v)))[0]!.o as { role: string; max: number }[]; const m: Record<string, number> = { lifestyle: 1 }; for (const o of owed) m[o.role] = Math.max(1, o.max); slots.set(listedAs(r.sku, r.name), m); }
const many = (role: string): boolean => angles.some(a => a.key === role && a.many === true);
const placedNow = await archive.query("SELECT i.role, i.build_row FROM build_image i WHERE i.build = $1::uuid AND i.build_row IS NOT NULL", b.id);
const taken = new Map<string, number>();
const labelById = new Map(rows.map(r => [r.id, listedAs(r.sku, r.name)]));
for (const i of placedNow) { const k = (labelById.get(String(i.build_row)) ?? "") + "\n" + String(i.role); taken.set(k, (taken.get(k) ?? 0) + 1); }
reads.sort((x, y) => y.p - x.p);
let placed = 0;
for (const r of reads) {
  if (r.sku === "") continue;
  const m = slots.get(r.sku) ?? {};
  let role = r.role;
  let slot = r.sku + "\n" + role;
  if ((m[role] ?? 0) === 0 || ((taken.get(slot) ?? 0) >= (m[role] ?? 0) && !many(role))) {
    if (role === "" || role === "video" || (m["detail"] ?? 0) === 0) continue;
    role = "detail";
    slot = r.sku + "\ndetail";
  }
  await archive.update("build_image", r.id, { build_row: rowOf.get(r.sku)!.id, role });
  taken.set(slot, (taken.get(slot) ?? 0) + 1);
  placed++;
}
if (placed > 0) await line(b.id, "vendor", "Placed " + placed + " of " + count(pool.length, "photo") + " by their file names");
return { matched: placed };
` },
  blank_build: { runs: 'vendors', source: PRE + BLANK },
  buyer_blank_build: { runs: 'buyers', source: PRE + BLANK },
  build_spec: { runs: 'vendors', source: PRE + SPEC },
  buyer_build_spec: { runs: 'buyers', source: PRE + SPEC },
  buyer_template_spec: { runs: 'buyers', source: String.raw`
const id = (input as { id: string }).id;
const r = await archive.query("SELECT template_spec($1::uuid) AS s", id);
return r[0]!.s;
` },
  import_file: { runs: 'buyers', source: PRE + CELL + String.raw`
const fileId = text("file");
if (fileId === "") refuse("file is required");
const ref = text("template");
const t = text("build") !== "" ? (await loadBuild()).template : ref !== "" ? String(((await archive.query("SELECT id FROM template WHERE id::text = $1 OR key = $1", ref))[0] ?? { id: "" }).id) : "";
if (!t) return refuse("There is no template to read the file against.");
const spec = await specOf(t);
const book = await readBook(fileId);
const { headers, rows } = parseGrid(pickSheet(book).rows);
if (headers.length === 0) refuse("Could not read " + book.name + ". Excel or CSV works best.");
const fields = spec.fields.filter(f => f.owner === "vendor");
const synonyms = ((await archive.query("SELECT synonyms FROM intake LIMIT 1"))[0]?.synonyms ?? {}) as Record<string, string[]>;
const { mapping, unknown } = exactMapping(fields, headers, synonyms);
const colTo: Record<string, string> = {};
for (const [k, h] of Object.entries(mapping)) colTo[h] = k;
let asked = false;
if (unknown.length > 0) {
  const remembered = await archive.query("SELECT column_map FROM build WHERE template = $1::uuid AND columns = $2::text[] ORDER BY number DESC LIMIT 1", t, headers);
  if (remembered[0]) { for (const [h, k] of Object.entries(remembered[0].column_map as Record<string, string>)) if (!colTo[h] && !Object.values(colTo).includes(k)) colTo[h] = k; }
  const left = unknown.filter(h => !colTo[h]);
  if (left.length > 0) {
    asked = true;
    const taken = new Set(Object.values(colTo));
    const said = await askHeaders(spec, fields, left, rows, taken);
    for (const h of left) { const a = said[h]; if (a && a.field !== "no field" && fields.some(f => f.key === a.field) && !taken.has(a.field)) { colTo[h] = a.field; taken.add(a.field); } }
  }
}
if (!Object.values(colTo).some(k => k === "manufacturer_sku" || k === "vendor_product_name")) refuse(book.name + " has no column we could read as the SKU or the product name. Add one and drop it again.");
const finalMap: Record<string, string> = {};
for (const [h, k] of Object.entries(colTo)) finalMap[k] = h;
return { source: book.name, headers, columns: headers, column_map: colTo, rows, mapping: finalMap, deterministic: !asked };
` },
  auto_import: { runs: 'vendors', source: PRE + CELL + String.raw`
const b = await openOf();
const spec = await specOf(b.template);
const fileId = text("file");
if (fileId === "") refuse("file is required");
const book = await readBook(fileId);
const { headers, rows } = parseGrid(pickSheet(book).rows);
if (headers.length === 0 || rows.length === 0) refuse("Could not read " + book.name + ". Excel or CSV works best.");
if (rows.length > 5000) refuse(book.name + " holds " + rows.length + " rows, more than Auto reads at once. Use Upload template on the Sheet page.");
const fields = spec.fields.filter(f => f.owner === "vendor");
const byKey = new Map(spec.fields.map(f => [f.key, f]));
const memory = await archive.query("SELECT kind, field, raw, value FROM vendor_answer WHERE vendor = $1::uuid", b.vendor);
const memHead = new Map(memory.filter(m => m.kind === "header").map(m => [squash(String(m.raw)), String(m.value)]));
const memCell = new Map(memory.filter(m => m.kind === "cell").map(m => [String(m.field) + "\n" + squash(String(m.raw)), String(m.value)]));
const colTo: Record<string, string> = {};
const taken = new Set<string>();
const labelled = new Map<string, string>();
for (const f of fields) labelled.set(squash(f.label), f.key);
for (const f of fields) if (!labelled.has(squash(f.key))) labelled.set(squash(f.key), f.key);
const synonyms = ((await archive.query("SELECT synonyms FROM intake LIMIT 1"))[0]?.synonyms ?? {}) as Record<string, string[]>;
const near = exactMapping(fields, headers, synonyms).mapping;
const left: string[] = [];
for (const h of headers) {
  const rem = memHead.get(squash(h));
  if (rem === "") continue;
  const k = rem ?? labelled.get(squash(h)) ?? Object.keys(near).find(x => near[x] === h);
  if (k && !taken.has(k) && byKey.has(k)) { colTo[h] = k; taken.add(k); continue; }
  left.push(h);
}
type Q = { id: string; kind: string; ask: string; raw: string; field: string; label: string; guess: string; options: { value: string; label: string; p?: number }[]; rows: number; sure: number };
const questions: Q[] = [];
const said = await askHeaders(spec, fields, left, rows, taken);
for (const h of [...left].sort((x, y) => (said[y]?.sure ?? 0) - (said[x]?.sure ?? 0))) {
  const a = said[h];
  if (!a || a.field === "no field") continue;
  const ranked = fields.filter(f => !taken.has(f.key) || f.key === a.field).map(f => ({ value: f.key, label: f.label, p: f.key === a.field ? a.sure : 0 })).sort((x, y) => y.p - x.p);
  if (taken.has(a.field) || !byKey.has(a.field)) { questions.push({ id: "", kind: "header", ask: "Which column is “" + h + "”?", raw: h, field: "column:" + h, label: "", guess: "", options: ranked, rows: rows.length, sure: a.sure }); continue; }
  colTo[h] = a.field;
  taken.add(a.field);
  if (a.sure < 0.7) questions.push({ id: "", kind: "header", ask: "Is “" + h + "” your " + (byKey.get(a.field)?.label ?? a.field) + "?", raw: h, field: a.field, label: byKey.get(a.field)?.label ?? a.field, guess: a.field, options: ranked, rows: rows.length, sure: a.sure });
}
const skuHead = Object.keys(colTo).find(h => colTo[h] === "manufacturer_sku");
const nameHead = Object.keys(colTo).find(h => colTo[h] === "vendor_product_name");
if (!skuHead && !nameHead) refuse(book.name + " has no column we could read as the SKU or the product name. Add one and drop it again.");
const stored = await rowsOf(b.id);
const bySku = new Map(stored.filter(r => r.sku !== "").map(r => [fold(r.sku), r]));
const byName = new Map(stored.filter(r => r.name !== "").map(r => [fold(r.name), r]));
const dropped: string[] = [];
const seen = new Set<string>();
const kept: { id: string; sku: string; line: Record<string, string> }[] = [];
for (const r of rows) {
  const hit = (skuHead ? bySku.get(fold(r[skuHead] ?? "")) : undefined) ?? (nameHead ? byName.get(fold(r[nameHead] ?? "")) : undefined);
  if (hit) { if (!seen.has(hit.id)) { seen.add(hit.id); kept.push({ id: hit.id, sku: hit.sku, line: r }); } continue; }
  const s = headers.map(h => (r[h] ?? "").trim()).filter(v => v !== "").slice(0, 3).join("  ");
  if (s !== "" && dropped.length < 12) dropped.push(s);
}
const counts = { columns: headers.length, mapped: Object.keys(colTo).length, dropped: headers.length - Object.keys(colTo).length, rows_read: rows.length, rows_kept: kept.length, cells: 0, free: 0, remembered: 0, auto: 0, asked: 0, requests: 0, input_tokens: 0, output_tokens: 0, cost: 0, millis: 0 };
const proposal: { row: string; sku: string; values: Record<string, string> }[] = kept.map(k => ({ row: k.id, sku: k.sku, values: {} }));
type Ask = { h: string; f: Field; raw: string; cells: number[] };
const toAsk = new Map<string, Ask>();
kept.forEach((k, i) => {
  for (const [h, key] of Object.entries(colTo)) {
    const raw = (k.line[h] ?? "").trim();
    if (raw === "") continue;
    const f = byKey.get(key);
    if (!f) continue;
    counts.cells++;
    if (f.type !== "select" && f.type !== "yesno") { proposal[i]!.values[key] = raw; counts.free++; continue; }
    const sp = spelledCell(f, raw);
    if (isSpelled(f, sp)) { proposal[i]!.values[key] = sp; counts.free++; continue; }
    const rem = memCell.get(key + "\n" + squash(raw));
    if (rem !== undefined) { counts.remembered++; if (rem !== "" && rem !== "␛clear") proposal[i]!.values[key] = rem; continue; }
    const q = key + "\n" + squash(raw);
    const a = toAsk.get(q) ?? { h, f, raw, cells: [] };
    a.cells.push(i);
    toAsk.set(q, a);
  }
});
const asks = [...toAsk.values()];
const reqs: Ask[][] = [];
for (let i = 0; i < asks.length; i += 10) reqs.push(asks.slice(i, i + 10));
const results = reqs.length === 0 ? [] : await archive.model(reqs.map(chunk => ({ model: MODEL, effort: "none" as const, system: "The vendor typed these cells out of their own system. For each, which option of the column did they mean? An abbreviation, initials, a truncation or a typo of an option is that option; units, counts and notes around it are not the answer. Answer with one option exactly as given, or \"none of these\". sure is how likely, 0 to 1.", prompt: JSON.stringify(chunk.map((a, n) => ({ n, column: a.f.label, options: a.f.type === "yesno" ? ["Yes", "No"] : a.f.options, cell: a.raw }))), schema: { type: "object", properties: { answers: { type: "array", items: { type: "object", properties: { n: { type: "number" }, option: { type: "string" }, sure: { type: "number" } }, required: ["n", "option", "sure"] } } }, required: ["answers"] } })));
counts.requests = reqs.length;
results.forEach((r, ri) => {
  counts.input_tokens += r.tokens_in; counts.output_tokens += r.tokens_out;
  for (const a of (r.json as { answers: { n: number; option: string; sure: number }[] }).answers) {
    const ask = reqs[ri]![a.n];
    if (!ask) continue;
    const opts = ask.f.type === "yesno" ? ["Yes", "No"] : ask.f.options ?? [];
    const valid = opts.includes(a.option);
    if (valid && a.sure >= 0.7) { for (const i of ask.cells) proposal[i]!.values[ask.f.key] = a.option; counts.auto += ask.cells.length; continue; }
    questions.push({ id: "", kind: "cell", ask: "What does “" + ask.raw + "” mean in " + ask.f.label + "?", raw: ask.raw, field: ask.f.key, label: ask.f.label, guess: valid ? a.option : "", options: opts.map(o => ({ value: o, label: o, p: o === a.option ? a.sure : 0 })), rows: ask.cells.length, sure: a.sure });
    for (const i of ask.cells) proposal[i]!.values[ask.f.key] = ask.raw;
  }
});
counts.asked = questions.length;
const read = counts.free + counts.remembered + counts.auto;
let notice = book.name + " fills " + proposal.length + " of " + count(stored.length, "product");
if (counts.cells > 0) notice += "; " + read + " of " + counts.cells + " cells read without asking";
if (questions.length > 0) notice += ", " + count(questions.length, "question") + " to settle";
return { source: book.name, rows: proposal, dropped, questions, counts, notice: notice + "." };
` },
  auto_confirm: { runs: 'vendors', source: PRE + CELL + String.raw`
const b = await openOf();
const spec = await specOf(b.template);
const byKey = new Map(spec.fields.map(f => [f.key, f]));
const answers = (Array.isArray(I["answers"]) ? I["answers"] : []) as { kind?: string; field?: string; raw?: string; value?: string }[];
const proposed = (Array.isArray(I["rows"]) ? I["rows"] : []) as { row?: string; sku?: string; values?: Record<string, string> }[];
const source = text("source");
const cells = new Map<string, Map<string, string>>();
const moves = new Map<string, string>();
for (const a of answers) {
  const field = String(a.field ?? "").trim(), raw = String(a.raw ?? "").trim(), value = String(a.value ?? "");
  if (field === "" || raw === "") continue;
  if (a.kind === "header") { if (value === field) continue; if (value !== "" && byKey.get(value)?.owner !== "vendor") refuse(value + " is not a field of this template."); moves.set(field, value); }
  else if (a.kind === "cell") { if (!byKey.has(field)) refuse(field + " is not a field of this template."); const m = cells.get(field) ?? new Map<string, string>(); m.set(squash(raw), value); cells.set(field, m); }
  else refuse("An answer is about a header or a cell.");
}
const stored = await rowsOf(b.id);
const byId = new Map(stored.map(r => [r.id, r]));
const bySku = new Map(stored.filter(r => r.sku !== "").map(r => [fold(r.sku), r.id]));
const done = new Set<string>();
let hit = 0;
for (const p of proposed) {
  let id = String(p.row ?? "").trim();
  if (!byId.has(id)) id = bySku.get(fold(String(p.sku ?? ""))) ?? "";
  if (id === "" || done.has(id)) continue;
  done.add(id);
  hit++;
  const held = byId.get(id)!;
  const moved: Record<string, string> = { ...(p.values ?? {}) };
  for (const [from, to] of moves) { const v = moved[from]; if (v === undefined) continue; delete moved[from]; if (to !== "") { const tf = byKey.get(to); moved[to] = tf ? spelledCell(tf, v) : v; } }
  for (const [field, m] of cells) { const h = moved[field]; if (!h || h.trim() === "") continue; const s = m.get(squash(h)); if (!s) continue; if (s === "␛clear") delete moved[field]; else moved[field] = s; }
  const write: Record<string, string | null> = {};
  for (const [key, raw] of Object.entries(moved)) {
    const f = byKey.get(key);
    if (!f || f.owner !== "vendor") continue;
    let v = raw;
    if (f.type === "select" || f.type === "yesno") { v = spelledCell(f, v); if (!isSpelled(f, v)) continue; }
    if (held.listed.includes(key)) continue;
    if ((held.v[key] ?? "") !== v) write[key] = v === "" ? null : v;
  }
  if (Object.keys(write).length > 0) await archive.update("build_row", id, write as never);
}
await line(b.id, who(), "Auto read " + source + " — " + hit + " of " + stored.length + " products filled");
for (const a of answers) {
  const raw = String(a.raw ?? "").trim();
  if (raw === "") continue;
  const kind = String(a.kind ?? ""), field = kind === "header" ? "" : String(a.field ?? "").trim(), value = String(a.value ?? "");
  const old = (await archive.query("SELECT id FROM vendor_answer WHERE vendor = $1::uuid AND kind = $2 AND field = $3 AND lower(btrim(raw)) = lower(btrim($4))", b.vendor, kind, field, raw))[0];
  if (old) await archive.update("vendor_answer", String(old.id), { value, decided_at: now() });
  else await archive.create("vendor_answer", { vendor: b.vendor, kind, field, raw, value, name: raw } as never);
}
return { updated: hit, notice: "Filled " + hit + " of " + count(stored.length, "product") + " from " + source + "." };
` },
  export_build: { runs: 'buyers', source: PRE + EXPORT },
  export_product: { runs: 'buyers', source: PRE + EXPORT },
  publish_template: { runs: 'buyers', source: PRE + String.raw`
const key = text("key");
if (key === "") refuse("key is required");
const def = (I["definition"] ?? {}) as { fields?: (Field & { must?: unknown[]; fills?: unknown[]; note?: string })[]; hides?: string[]; images?: unknown[] | null; photo?: Row | null; export?: Row };
const exists = await archive.query("SELECT id, name, parent FROM template WHERE key = $1", key);
const prior = exists[0];
const pRef = text("parent");
const parentRow = pRef !== "" ? (await archive.query("SELECT id FROM template WHERE id::text = $1 OR key = $1", pRef))[0] : undefined;
if (pRef !== "" && !parentRow) refuse("no template named " + pRef + " to inherit from");
let parent = parentRow ? String(parentRow.id) : prior ? (prior.parent as string | null) : null;
if (!prior && parent === null) { const root = await archive.query("SELECT id FROM template WHERE parent IS NULL LIMIT 1"); if (root[0]) parent = String(root[0].id); }
const name = text("name") || (prior ? String(prior.name) : "");
if (name.trim() === "") refuse("a template needs a name");
const given = def.fields ?? [];
for (const f of given) {
  if (!/^[a-z_][a-z0-9_]*$/.test(f.key)) refuse(JSON.stringify(f.key) + " is not a field key: lowercase letters, digits and _.");
  if (!["text", "number", "select", "yesno", "country", "dimensions"].includes(f.type)) refuse((f.label || f.key) + " has no type we know.");
  if (f.type === "select" && (f.options ?? []).length === 0) refuse((f.label || f.key) + " is a choice with no options.");
  if (f.owner !== "vendor" && f.owner !== "lg") refuse((f.label || f.key) + " must be owned by the vendor or lg.");
}
const parentSpec = parent ? await specOf(parent) : null;
const pf = new Map((parentSpec?.fields ?? []).map(f => [f.key, JSON.stringify(f)]));
const stableKeys = (v: unknown): string => { if (Array.isArray(v)) return "[" + v.map(stableKeys).join(",") + "]"; if (v && typeof v === "object") { const o = v as Record<string, unknown>; return "{" + Object.keys(o).sort().map(k => JSON.stringify(k) + ":" + stableKeys(o[k])).join(",") + "}"; } return JSON.stringify(v) ?? "null"; };
const own = given.filter(f => { const p = pf.get(f.key); return !p || stableKeys(JSON.parse(p)) !== stableKeys(f); });
const hides = (parentSpec?.fields ?? []).map(f => f.key).filter(k => !given.some(f => f.key === k));
const row = prior ? await archive.update("template", String(prior.id), { name, parent, hides, images: def.images ?? null, photo: def.photo ?? null, export: def.export ?? {}, ...(text("icon") ? { icon: text("icon") } : {}), ...(text("sheet") ? { sheet: text("sheet") } : {}) } as never) : await archive.create("template", { key, name, parent, hides, images: def.images ?? null, photo: def.photo ?? null, export: def.export ?? {}, icon: text("icon"), sheet: text("sheet") || "VENDOR FILL" } as never);
const tid = (row as { id: string }).id;
const cols = new Set((await archive.query("SELECT a.attname::text AS c FROM pg_catalog.pg_attribute a WHERE a.attrelid = 'public.build_row'::regclass AND a.attnum > 0 AND NOT a.attisdropped")).map(r => String(r.c)));
const fresh = own.filter(f => !cols.has(f.key));
if (fresh.length > 0) {
  const q = (s: string): string => "'" + s.replace(/'/g, "''") + "'";
  const sql = "ALTER TABLE build_row " + fresh.map(f => "ADD COLUMN " + f.key + " text").join(", ") + ";\n" + fresh.map(f => "COMMENT ON COLUMN build_row." + f.key + " IS " + q(f.label + "\nsection: " + f.section + "\ntype: " + f.type + "\nowner: " + f.owner + "\ntemplates: " + key)).join(";\n") + ";\n" + fresh.filter(f => f.owner === "vendor").map(f => "GRANT SELECT (" + f.key + "), UPDATE (" + f.key + ") ON build_row TO guest").join(";\n") + (fresh.some(f => f.owner === "vendor") ? ";" : "");
  const m = await archive.migrate(sql);
  await archive.migrate(sql, { confirm: m.confirm });
}
for (const old of await archive.list("field", { where: { template: tid } as never, limit: 2000 })) await archive.delete("field", old.id);
let position = 0;
for (const f of own) { await archive.create("field", { template: tid, key: f.key, name: f.label, section: f.section ?? "", type: f.type, owner: f.owner, required: !!f.required, required_when: f.requiredWhen ?? null, must: f.must ?? null, fills: f.fills ?? null, options: f.options ?? null, parts: f.parts ?? null, help: f.help ?? null, note: f.note ?? null, multi: !!f.multi, pct_breakdown: !!f.pctBreakdown, position } as never); position++; }
const out: Row = { id: tid, key, name };
if (parent) out["parent"] = parent;
if (prior) {
  const open = await archive.query("WITH RECURSIVE under AS (SELECT id FROM template WHERE id = $1::uuid UNION ALL SELECT t.id FROM template t JOIN under u ON t.parent = u.id) SELECT b.id, b.errors, b.stage FROM build b WHERE b.template IN (SELECT id FROM under) AND b.stage NOT IN ('complete', 'cancelled')", tid);
  for (const b of open) {
    await archive.update("build", String(b.id), { restamp_at: now() });
    const after = await archive.get("build", String(b.id));
    if (after.errors > Number(b.errors) && (after.stage === "sent" || after.stage === "in_progress")) { const left = after.skus - after.clean; const bad = await mail("reminder", { build: after.id }); if (bad === "") await archive.update("build", after.id, { reminders_sent: after.reminders_sent + 1, last_reminded_at: now() }); await line(after.id, "system", bad === "" ? "Reminder sent: the template now asks for more: " + left + " SKU" + plural(left) + " still need attention" : "The reminder could not be emailed to " + after.emails.join(", ")); }
  }
  out["restamped"] = open.length;
}
return out;
` },
  duplicate_template: { runs: 'buyers', source: PRE + String.raw`
const ref = text("id");
const t = (await archive.query("SELECT id, key, name, parent, icon, sheet, shell, hides, images, photo, export FROM template WHERE id::text = $1 OR key = $1", ref))[0];
if (!t) return refuse("no such template");
const key = String(t.key) + "_copy_" + Date.now().toString(36);
const c = made<{ id: string }>(await archive.create("template", { key, name: String(t.name) + " copy", parent: t.parent, icon: t.icon, sheet: t.sheet, shell: t.shell, hides: t.hides, images: t.images, photo: t.photo, export: t.export } as never));
for (const f of await archive.list("field", { where: { template: String(t.id) } as never, limit: 2000 })) await archive.create("field", { template: c.id, key: f.key, name: f.name, section: f.section, type: f.type, owner: f.owner, required: f.required, required_when: f.required_when, must: f.must, fills: f.fills, options: f.options, parts: f.parts, help: f.help, note: f.note, multi: f.multi, pct_breakdown: f.pct_breakdown, position: f.position } as never);
return { id: c.id, key, name: String(t.name) + " copy", ...(t.parent ? { parent: t.parent } : {}) };
` },
};

const SIMPLE = {
  notify_review: { runs: null, on: { table: 'build_activity', events: ['created'] }, source: String.raw`
for (const row of input.created) {
  if (!/^(Complete|Submitted): /.test(row.text)) continue;
  const r = await archive.run("mail", { kind: "review", build: row.build });
  if (r.error !== "") console.error(r.error);
}
return { seen: input.created.length };
` },
  judge_cells: { runs: null, on: { table: 'build_row', events: ['created', 'updated'] }, source: String.raw`
const rows = input.rows;
if (rows.length === 0) return { asked: 0 };
const owed = await archive.query("WITH r AS (SELECT * FROM build_row WHERE id = ANY($1::uuid[])) SELECT DISTINCT e ->> 'key' AS field, r.build::text AS build, coalesce(e ->> 'label', '') AS label, coalesce(e ->> 'help', '') AS help, btrim(row_values(r) ->> (e ->> 'key')) AS raw, t.name AS template FROM r JOIN build b ON b.id = r.build JOIN template t ON t.id = b.template CROSS JOIN LATERAL jsonb_array_elements(template_spec(b.template) -> 'fields') e WHERE e ->> 'owner' = 'vendor' AND e ->> 'type' = 'text' AND e ->> 'key' NOT IN ('manufacturer_sku', 'vendor_product_name') AND coalesce(btrim(row_values(r) ->> (e ->> 'key')), '') <> '' AND octet_length(btrim(row_values(r) ->> (e ->> 'key'))) <= 60 AND ((e ->> 'required')::boolean IS TRUE OR jsonb_array_length(coalesce(e -> 'requiredWhen', '[]')) > 0) AND NOT EXISTS (SELECT FROM cell_answer c WHERE c.field = e ->> 'key' AND lower(btrim(c.raw)) = lower(btrim(row_values(r) ->> (e ->> 'key'))))", rows.map(r => r.id));
if (owed.length === 0) return { asked: 0 };
const words = [...new Map(owed.map(o => [String(o.field) + "|" + String(o.raw).toLowerCase(), o])).values()];
const reqs: (typeof words)[] = [];
for (let i = 0; i < words.length; i += 10) reqs.push(words.slice(i, i + 10));
const ask = async () => archive.model(reqs.map(chunk => ({ model: "claude-haiku-5-5", effort: "none" as const, system: "A vendor filled these fields in for their products. Does what they wrote give the field a value, rather than a placeholder that skips it? answers is true for a value for the field, however short, misspelled or abbreviated, and for None or No where the product really has none of it; false for a placeholder that says nothing about the product: N/A, not captured, not applicable, not available, TBD, unknown, see photo, ask factory, a dash or a question mark. sure is how likely you are right, 0 to 1.", prompt: JSON.stringify(chunk.map((w, n) => ({ n, field: w.label, "what the field asks": w.help, "the vendor wrote": w.raw }))), schema: { type: "object", properties: { answers: { type: "array", items: { type: "object", properties: { n: { type: "number" }, answers: { type: "boolean" }, sure: { type: "number" } }, required: ["n", "answers", "sure"] } } }, required: ["answers"] } })));
let out: ModelResult[];
try { out = await ask(); } catch (e) { console.warn(String(e)); return { asked: 0 }; }
const blank = new Set<string>();
for (let i = 0; i < out.length; i++) {
  for (const a of (out[i]!.json as { answers: { n: number; answers: boolean; sure: number }[] }).answers) {
    const w = reqs[i]![a.n];
    if (!w) continue;
    const says = a.answers || a.sure < 0.7;
    await archive.create("cell_answer", { field: String(w.field), raw: String(w.raw), answers: says, name: String(w.raw) } as never);
    if (!says) blank.add(String(w.build));
  }
}
for (const id of blank) await archive.update("build", id, { restamp_at: new Date().toISOString() });
return { asked: words.length, blank: blank.size };
` },
};

const CONTRACT_COLUMNS = {
  build_book: 'id, number, name, vendor, vendor_slug, vendor_id, category, category_name, category_icon, template, stage, status, standing, tone, step, progress, skus, clean, data_clean, photos_clean, wants_photos, photos, photos_placed, errors, warns, emails, created_by, created_by_name, due_at, remind_every_days, reminders_sent, sent_at, opened_at, submitted_at, returned_at, approved_at, cancelled_at, last_activity_at, vendor_active_at, created_at',
  vendor_book: 'id, name, slug, country, contacts, contact_count, open_builds, builds, last_sent_at, has_link, link_minted_at, created_at',
  template_book: 'id, key, name, parent, parent_key, parent_name, icon, sheet, shell, open_builds, children, updated_at, created_at',
  build_sku: 'id, name, build, build_number, build_name, vendor, sku, product, done, errors, warns, owes, notes, why, photos, on_builds, lead_time, ship_method, position',
  sku_elsewhere: 'id, build, sku, name, other_build, other_number, other_name, other_vendor, other_stage',
  portal_book: 'id, number, name, vendor, vendor_id, category, category_name, category_icon, template, stage, skus, clean, data_clean, photos_clean, wants_photos, photos, photos_placed, errors, warns, due_at, sent_at, opened_at, submitted_at, returned_at, return_note, approved_at, cancelled_at, source, created_at',
};

const upsertWorkflows = async defs => {
  const names = Object.keys(defs);
  for (let i = 0; i < names.length; i += 3) {
    const chunk = names.slice(i, i + 3).map(name => ({ name, source: defs[name].source, runs: defs[name].runs, on: defs[name].on ?? null }));
    const out = await run(`const defs = JSON.parse(${JSON.stringify(JSON.stringify(chunk))}) as { name: string; source: string; runs: string | null; on: unknown }[];
const groups = await archive.query('SELECT id, name FROM archive."group"');
const said: string[] = [];
for (const d of defs) {
  const runs = d.runs === null ? null : groups.find(g => g.name === d.runs)?.id ?? null;
  const had = (await archive.query("SELECT id FROM archive.workflow WHERE name = $1", d.name))[0];
  if (had) await archive.update("workflow", String(had.id), { source: d.source, runs, on: d.on } as never);
  else await archive.create("workflow", { name: d.name, source: d.source, runs, on: d.on } as never);
  said.push(d.name);
}
return said;`);
    console.log('saved', out.join(', '));
  }
};

const install = async () => {
  const only = process.env.MAIL_ONLY;
  if (!only) throw new Error('MAIL_ONLY: want the one address mail may go to while this workspace is tested');
  await run(`const said = ${JSON.stringify(only)}; const had = await archive.list("mail_only"); if (!had.some(m => String(m.email).toLowerCase() === said.toLowerCase())) await archive.create("mail_only", { email: said }); return 1;`);
  await migrate('lulu_schema_1', SCHEMA);
  await migrate('lulu_schema_2', MORE);
  await migrate('lulu_schema_3', MORE3);
  await versioned('lulu_functions', FUNCTIONS);
  await versioned('lulu_triggers', TRIGGERS);
  await versioned('lulu_views', VIEWS);
  await run(`for (const name of ["buyers", "vendors"]) { const had = await archive.query('SELECT id FROM archive."group" WHERE name = $1', name); if (had.length === 0) await archive.create("group", { name } as never); }
const a = await archive.query("SELECT id FROM archive.agent WHERE slug = 'intake'");
if (a.length === 0) await archive.create("agent", { name: "Intake", description: "Sends the vendor desk's mail.", instructions: "You send the vendor desk's mail.", avatar: 0, tint: 0 } as never);
return 1;`);
  const contracts = Object.fromEntries(Object.entries(CONTRACT_COLUMNS).map(([v, cols]) => [`contract_${v}`, { runs: null, source: `const rows = await archive.query("SELECT ${cols} FROM ${v} LIMIT 0");\nreturn rows.length;\n` }]));
  await upsertWorkflows({ ...WF, ...SIMPLE, ...contracts });
};

export { run, migrate, versioned, API, headers, SCHEMA, FUNCTIONS, TRIGGERS, VIEWS, MORE, WF, SIMPLE, upsertWorkflows };

if (process.argv[1] === new URL(import.meta.url).pathname) await install();
