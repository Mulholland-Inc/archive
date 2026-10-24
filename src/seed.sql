INSERT INTO types (name, extends, def, builtin)
SELECT name, extends, def::jsonb, true FROM (VALUES
('date', '', '{
  "description": "Calendar date, ISO 8601", "category": "time",
  "schema": {"type": "string", "format": "date", "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
  "parse": "date", "check": "date", "format": "date",
  "examples": ["2026-10-23"]}'),
('timestamp', '', '{
  "description": "Instant in UTC, with the IANA zone it was given in", "category": "time",
  "schema": {"type": "object", "properties": {
    "utc": {"type": "string", "format": "date-time", "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$"},
    "zone": {"type": "string"}}, "required": ["utc", "zone"], "additionalProperties": false},
  "parse": "timestamp", "check": "timestamp", "format": "timestamp",
  "examples": [{"utc": "2026-10-23T12:30:00Z", "zone": "Europe/Amsterdam"}, {"utc": "2026-10-23T12:30:00.5Z", "zone": "UTC"}]}'),
('time', '', '{
  "description": "Time of day, with an optional IANA zone", "category": "time",
  "schema": {"type": "object", "properties": {
    "time": {"type": "string", "pattern": "^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$"},
    "zone": {"anyOf": [{"type": "string"}, {"type": "null"}]}}, "required": ["time", "zone"], "additionalProperties": false},
  "parse": "time", "check": "time", "format": "time",
  "examples": [{"time": "14:30:00", "zone": "Europe/Amsterdam"}, {"time": "09:00:00", "zone": null}]}'),
('duration', '', '{
  "description": "ISO 8601 duration", "category": "time",
  "schema": {"type": "string", "pattern": "^P([0-9]+Y)?([0-9]+M)?([0-9]+D)?(T([0-9]+H)?([0-9]+M)?([0-9]+(\\.[0-9]+)?S)?)?$"},
  "parse": "duration", "check": "duration",
  "examples": ["PT1H30M", "P1Y2M10DT2H", "PT0.5S"]}'),
('interval', '', '{
  "description": "Start and end, each a date or UTC instant; null is open", "category": "time",
  "schema": {"type": "object", "properties": {
    "start": {"anyOf": [{"type": "string", "format": "date"}, {"type": "string", "format": "date-time"}, {"type": "null"}]},
    "end": {"anyOf": [{"type": "string", "format": "date"}, {"type": "string", "format": "date-time"}, {"type": "null"}]}},
    "required": ["start", "end"], "additionalProperties": false},
  "parse": "interval", "check": "interval", "format": "interval",
  "examples": [{"start": "2026-01-01", "end": "2026-12-31"}, {"start": "2026-10-23T08:00:00Z", "end": null}]}'),
('fuzzy_date', '', '{
  "description": "A date as people know it, exact or not: about 1890, before 1900, between 1890 and 1895, March 1890, the 1890s. EDTF with the earliest and latest day it can mean; text no date can be read from is kept as text", "category": "time",
  "schema": {"type": "object", "properties": {
    "edtf": {"anyOf": [{"type": "string"}, {"type": "null"}]},
    "min": {"anyOf": [{"type": "string", "format": "date"}, {"type": "null"}]},
    "max": {"anyOf": [{"type": "string", "format": "date"}, {"type": "null"}]},
    "text": {"anyOf": [{"type": "string"}, {"type": "null"}]}},
    "required": ["edtf", "min", "max", "text"], "additionalProperties": false},
  "parse": "fuzzy_date", "check": "fuzzy_date", "format": "fuzzy_date",
  "examples": [{"edtf": "1890~", "min": "1890-01-01", "max": "1890-12-31", "text": null}, {"edtf": "../1900", "min": null, "max": "1900-12-31", "text": null}, {"edtf": null, "min": null, "max": null, "text": "the spring after the flood"}]}'),
('timezone', '', '{
  "description": "IANA time zone", "category": "time",
  "schema": {"type": "string", "pattern": "^[A-Za-z][A-Za-z0-9_+\\-/]*$"},
  "normalize": ["timezone"],
  "examples": ["Europe/Amsterdam", "UTC"]}'),
('recurrence', '', '{
  "description": "RFC 5545 recurrence rule", "category": "time",
  "schema": {"type": "string", "pattern": "^FREQ=(SECONDLY|MINUTELY|HOURLY|DAILY|WEEKLY|MONTHLY|YEARLY)(;[A-Z]+=[A-Z0-9,+\\-]+)*$"},
  "normalize": ["rrule"],
  "examples": ["FREQ=WEEKLY;BYDAY=MO", "FREQ=MONTHLY;COUNT=3;BYMONTHDAY=-1"]}'),

('money', '', '{
  "description": "Amount as a decimal string in the currency minor units, never a float", "category": "money",
  "fields": {"amount": {"type": "decimal", "required": true}, "currency": {"type": "currency", "required": true}},
  "parse": "money", "check": "money", "format": "money",
  "examples": [{"amount": "1234.50", "currency": "EUR"}, {"amount": "500", "currency": "JPY"}, {"amount": "-0.125", "currency": "BHD"}]}'),
('currency', '', '{
  "description": "ISO 4217 currency code", "category": "money",
  "schema": {"type": "string", "pattern": "^[A-Z]{3}$"},
  "parse": "currency", "check": "currency",
  "examples": ["EUR", "JPY"]}'),
('percentage', '', '{
  "description": "Fraction, 0.21 is 21%", "category": "money",
  "schema": {"type": "number"},
  "parse": "percentage", "format": "percentage",
  "examples": [0.21, 1.5, 0]}'),

('iban', '', '{
  "description": "ISO 13616 bank account number", "category": "finance", "sensitivity": "personal",
  "schema": {"type": "string", "pattern": "^((NO)[0-9]{2}[A-Z0-9]{11}|(BE)[0-9]{2}[A-Z0-9]{12}|(DK|FI|FK|FO|GL|NL|SD)[0-9]{2}[A-Z0-9]{14}|(MK|SI)[0-9]{2}[A-Z0-9]{15}|(AT|BA|EE|KZ|LT|LU|MN|XK)[0-9]{2}[A-Z0-9]{16}|(CH|HR|LI|LV)[0-9]{2}[A-Z0-9]{17}|(BG|BH|CR|DE|GB|GE|IE|ME|RS|VA)[0-9]{2}[A-Z0-9]{18}|(AE|GI|IL|IQ|OM|SO|TL)[0-9]{2}[A-Z0-9]{19}|(AD|CZ|ES|MD|PK|RO|SA|SE|SK|TN|VG)[0-9]{2}[A-Z0-9]{20}|(LY|PT|ST)[0-9]{2}[A-Z0-9]{21}|(IS|TR)[0-9]{2}[A-Z0-9]{22}|(BI|DJ|FR|GR|IT|MC|MR|SM)[0-9]{2}[A-Z0-9]{23}|(AL|AZ|BY|CY|DO|GT|HU|LB|NI|PL|SV)[0-9]{2}[A-Z0-9]{24}|(BR|EG|PS|QA|UA)[0-9]{2}[A-Z0-9]{25}|(JO|KW|MU|YE)[0-9]{2}[A-Z0-9]{26}|(MT|SC)[0-9]{2}[A-Z0-9]{27}|(LC)[0-9]{2}[A-Z0-9]{28}|(RU)[0-9]{2}[A-Z0-9]{29})$"},
  "normalize": ["strip_separators", "upper"], "check": "mod97", "format": "group4",
  "examples": ["NL91ABNA0417164300", "DE89370400440532013000"]}'),
('bic', '', '{
  "description": "ISO 9362 bank identifier", "category": "finance",
  "schema": {"type": "string", "pattern": "^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$"},
  "normalize": ["strip_separators", "upper", "bic"],
  "examples": ["ABNANL2A", "DEUTDEFF500"]}'),
('card', '', '{
  "description": "Payment card: a SHA-256 token, the brand and the last four digits; the number is never stored", "category": "finance", "sensitivity": "secret",
  "schema": {"type": "object", "properties": {
    "token": {"type": "string", "pattern": "^[0-9a-f]{64}$"},
    "brand": {"type": "string", "enum": ["visa", "mastercard", "amex", "discover", "diners", "jcb", "unionpay", "maestro", "unknown"]},
    "last4": {"type": "string", "pattern": "^[0-9]{4}$"}}, "required": ["token", "brand", "last4"], "additionalProperties": false},
  "parse": "card", "format": "card",
  "examples": [{"token": "abababababababababababababababababababababababababababababababab", "brand": "visa", "last4": "4242"}]}'),
('us_routing', '', '{
  "description": "US ABA routing number", "category": "finance",
  "schema": {"type": "string", "pattern": "^[0-9]{9}$"},
  "normalize": ["strip_separators"], "check": "aba",
  "examples": ["011000015", "021000021"]}'),
('lei', '', '{
  "description": "ISO 17442 legal entity identifier", "category": "finance",
  "schema": {"type": "string", "pattern": "^[A-Z0-9]{18}[0-9]{2}$"},
  "normalize": ["strip_separators", "upper"], "check": "lei",
  "examples": ["5493001KJTIIGC8Y1R12"]}'),
('isin', '', '{
  "description": "ISO 6166 securities identifier", "category": "finance",
  "schema": {"type": "string", "pattern": "^[A-Z]{2}[A-Z0-9]{9}[0-9]$"},
  "normalize": ["strip_separators", "upper"], "check": "isin",
  "examples": ["US0378331005", "NL0000235190"]}'),

('vat_id', '', '{
  "description": "EU or UK VAT number with its country prefix", "category": "tax",
  "schema": {"type": "string", "pattern": "^(ATU[0-9]{8}|BE[01][0-9]{9}|BG[0-9]{9,10}|CY[0-9]{8}[A-Z]|CZ[0-9]{8,10}|DE[0-9]{9}|DK[0-9]{8}|EE[0-9]{9}|EL[0-9]{9}|ES[A-Z0-9][0-9]{7}[A-Z0-9]|FI[0-9]{8}|FR[A-HJ-NP-Z0-9]{2}[0-9]{9}|HR[0-9]{11}|HU[0-9]{8}|IE([0-9]{7}[A-W][A-I]?|[0-9][A-Z+*][0-9]{5}[A-W])|IT[0-9]{11}|LT([0-9]{9}|[0-9]{12})|LU[0-9]{8}|LV[0-9]{11}|MT[0-9]{8}|NL[0-9]{9}B[0-9]{2}|PL[0-9]{10}|PT[0-9]{9}|RO[0-9]{2,10}|SE[0-9]{12}|SI[0-9]{8}|SK[0-9]{10}|(GB|XI)([0-9]{9}|[0-9]{12}|GD[0-9]{3}|HA[0-9]{3}))$"},
  "normalize": ["strip_separators", "upper"], "parse": "vat_id",
  "examples": ["NL123456789B01", "DE123456789", "ATU12345678"]}'),
('us_ein', '', '{
  "description": "US employer identification number", "category": "tax",
  "schema": {"type": "string", "pattern": "^(0[1-6]|1[0-6]|2[0-7]|3[0-9]|4[0-8]|5[0-9]|6[0-8]|7[1-7]|8[0-8]|9[0-58-9])-[0-9]{7}$"},
  "normalize": ["strip_separators", "ein_dash"],
  "examples": ["12-3456789"]}'),
('us_ssn', '', '{
  "description": "US social security number", "category": "identity", "sensitivity": "secret",
  "schema": {"type": "string", "pattern": "^[0-9]{9}$"},
  "normalize": ["strip_separators"], "check": "ssn_ranges", "format": "mask_ssn",
  "examples": ["123456789"]}'),
('nl_bsn', '', '{
  "description": "Dutch citizen service number", "category": "identity", "sensitivity": "secret",
  "schema": {"type": "string", "pattern": "^[0-9]{9}$"},
  "normalize": ["strip_separators", "bsn9"], "check": "elfproef", "format": "mask",
  "examples": ["111222333", "123456782"]}'),
('national', '', '{
  "description": "National id number with its country and scheme", "category": "identity", "sensitivity": "secret",
  "fields": {
    "country": {"type": "country", "required": true},
    "scheme": {"type": "text", "required": true, "enum": ["bsn", "ssn", "nino", "idnr", "nir", "rrn"]},
    "value": {"type": "text", "required": true}},
  "parse": "national", "check": "national", "format": "national",
  "examples": [
    {"country": "NL", "scheme": "bsn", "value": "111222333"},
    {"country": "GB", "scheme": "nino", "value": "AB123456C"},
    {"country": "DE", "scheme": "idnr", "value": "86095742719"},
    {"country": "FR", "scheme": "nir", "value": "255081416802538"},
    {"country": "BE", "scheme": "rrn", "value": "85073003328"}]}'),
('passport', '', '{
  "description": "Passport number with its issuing country", "category": "identity", "sensitivity": "secret",
  "fields": {"country": {"type": "country", "required": true}, "number": {"type": "text", "required": true}},
  "parse": "passport", "check": "passport", "format": "passport",
  "examples": [{"country": "NL", "number": "NW92BC7P1"}]}'),

('email', '', '{
  "description": "Email address; the domain is lowercase and punycode", "category": "contact", "sensitivity": "personal",
  "schema": {"type": "string", "pattern": "^[^@\\s]+@[^@\\s]+$"},
  "normalize": ["mail_address", "lower_domain"], "format": "email",
  "examples": ["jan@example.com", "Jan.Jansen@xn--mnchen-3ya.de"]}'),
('phone', '', '{
  "description": "Phone number in E.164", "category": "contact", "sensitivity": "personal",
  "schema": {"type": "string", "pattern": "^\\+[1-9][0-9]{6,14}$"},
  "parse": "phone", "check": "phone", "format": "phone",
  "examples": ["+31612345678", "+12025550123"]}'),
('address', '', '{
  "description": "Postal address", "category": "contact", "sensitivity": "personal",
  "fields": {
    "lines": {"type": "text", "list": true, "required": true},
    "locality": {"type": "text"},
    "region": {"type": "region"},
    "postcode": {"type": "text"},
    "country": {"type": "country", "required": true}},
  "parse": "address", "check": "address", "format": "address",
  "examples": [
    {"lines": ["Damrak 1"], "locality": "Amsterdam", "region": "NL-NH", "postcode": "1012 LG", "country": "NL"},
    {"lines": ["1600 Pennsylvania Ave NW"], "locality": "Washington", "region": "US-DC", "postcode": "20500", "country": "US"},
    {"lines": ["10 Downing Street"], "locality": "London", "region": null, "postcode": "SW1A 2AA", "country": "GB"}]}'),
('name', '', '{
  "description": "Person name: the full original, and its parts when known", "category": "contact", "sensitivity": "personal",
  "fields": {"full": {"type": "text", "required": true}, "given": {"type": "text"}, "family": {"type": "text"}, "prefixes": {"type": "text"}},
  "parse": "name", "check": "name", "format": "name",
  "examples": [
    {"full": "Jan van der Berg", "given": "Jan", "family": "Berg", "prefixes": "van der"},
    {"full": "毛泽东", "given": null, "family": null, "prefixes": null}]}'),
('url', '', '{
  "description": "Web address, WHATWG-normalized", "category": "contact",
  "schema": {"type": "string", "format": "uri"},
  "parse": "url", "check": "url",
  "examples": ["https://example.com/", "https://xn--mnchen-3ya.de/a?b=1#c"]}'),

('country', '', '{
  "description": "ISO 3166-1 alpha-2 country code", "category": "place",
  "schema": {"type": "string", "pattern": "^[A-Z]{2}$"},
  "normalize": ["country"], "format": "country",
  "examples": ["NL", "US"]}'),
('region', '', '{
  "description": "ISO 3166-2 subdivision code", "category": "place",
  "schema": {"type": "string", "pattern": "^[A-Z]{2}-[A-Z0-9]{1,3}$"},
  "parse": "region", "check": "region",
  "examples": ["NL-NH", "US-CA"]}'),
('postcode', '', '{
  "description": "Postcode with the country it is valid in", "category": "place",
  "fields": {"country": {"type": "country", "required": true}, "code": {"type": "text", "required": true}},
  "parse": "postcode", "check": "postcode", "format": "postcode",
  "examples": [{"country": "NL", "code": "1012 LG"}, {"country": "US", "code": "90210"}, {"country": "GB", "code": "SW1A 2AA"}]}'),
('geo', '', '{
  "description": "WGS 84 latitude and longitude", "category": "place", "sensitivity": "personal",
  "fields": {"lat": {"type": "number", "required": true}, "lon": {"type": "number", "required": true}},
  "parse": "geo", "check": "geo", "format": "geo",
  "examples": [{"lat": 52.3731, "lon": 4.8926}]}'),
('locale', '', '{
  "description": "BCP 47 language tag", "category": "language",
  "schema": {"type": "string", "pattern": "^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$"},
  "normalize": ["locale"],
  "examples": ["nl-NL", "en-US", "zh-Hant-TW"]}'),

('quantity', '', '{
  "description": "Value in the SI base unit, with the UCUM unit it was given in", "category": "measure",
  "fields": {"value": {"type": "number", "required": true}, "unit": {"type": "text", "required": true}, "original_unit": {"type": "text", "required": true}},
  "parse": "quantity", "check": "quantity", "format": "quantity",
  "examples": [{"value": 1.5, "unit": "m", "original_unit": "cm"}, {"value": 293.15, "unit": "K", "original_unit": "Cel"}]}'),
('gtin', '', '{
  "description": "GS1 trade item number as 14 digits; accepts EAN-13 and UPC-A", "category": "product",
  "schema": {"type": "string", "pattern": "^[0-9]{14}$"},
  "normalize": ["strip_separators", "gtin14"], "check": "gtin",
  "examples": ["08712345678906", "00012345678905"]}'),
('isbn', '', '{
  "description": "ISBN-13; ISBN-10 is converted", "category": "product",
  "schema": {"type": "string", "pattern": "^97[89][0-9]{10}$"},
  "normalize": ["strip_separators", "upper", "isbn13"], "check": "isbn",
  "examples": ["9780306406157"]}'),

('uuid', '', '{
  "description": "RFC 9562 UUID, lowercase", "category": "technical",
  "schema": {"type": "string", "pattern": "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"},
  "normalize": ["uuid"],
  "examples": ["0192e4a0-7b1c-7cc3-9d2e-3f4a5b6c7d8e"]}'),
('ip', '', '{
  "description": "IPv4 or IPv6 address, RFC 5952 text", "category": "technical",
  "schema": {"anyOf": [{"type": "string", "format": "ipv4"}, {"type": "string", "format": "ipv6"}]},
  "normalize": ["ip"],
  "examples": ["192.0.2.1", "2001:db8::1"]}'),
('domain', '', '{
  "description": "Domain name, lowercase punycode", "category": "technical",
  "schema": {"type": "string", "pattern": "^[a-z0-9]([a-z0-9.\\-]*[a-z0-9])?$"},
  "normalize": ["punycode"], "format": "domain",
  "examples": ["example.com", "xn--mnchen-3ya.de"]}'),
('mime', '', '{
  "description": "IANA media type", "category": "technical",
  "schema": {"type": "string", "pattern": "^[a-z]+/[a-z0-9!#$&^_.+\\-]+(;.*)?$"},
  "normalize": ["mime"],
  "examples": ["application/pdf", "text/plain; charset=utf-8"]}'),
('file', '', '{
  "description": "File by content hash, with its name and media type", "category": "technical",
  "fields": {"sha256": {"type": "text", "required": true}, "name": {"type": "text", "required": true}, "mime": {"type": "mime", "required": true}},
  "parse": "file", "check": "file", "format": "file",
  "examples": [{"sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "name": "lease.pdf", "mime": "application/pdf"}]}'),

('text', '', '{
  "description": "Plain text", "category": "basic",
  "schema": {"type": "string"},
  "normalize": ["nfc"],
  "examples": ["Hello, world"]}'),
('richtext', 'text', '{
  "description": "Markdown (CommonMark)",
  "examples": ["**Hello**, _world_"]}'),
('integer', '', '{
  "description": "Whole number within 2^53", "category": "basic",
  "schema": {"type": "integer"},
  "parse": "number", "format": "number",
  "examples": [42, -7, 0]}'),
('number', '', '{
  "description": "Number", "category": "basic",
  "schema": {"type": "number"},
  "parse": "number", "format": "number",
  "examples": [3.5, -0.25, 0]}'),
('decimal', '', '{
  "description": "Exact decimal as a string", "category": "basic",
  "schema": {"type": "string", "pattern": "^-?(0|[1-9][0-9]*)(\\.[0-9]+)?$"},
  "parse": "decimal", "format": "number",
  "examples": ["3.14", "-0.5", "100"]}'),
('boolean', '', '{
  "description": "True or false", "category": "basic",
  "schema": {"type": "boolean"},
  "parse": "boolean",
  "examples": [true, false]}'),
('enum', '', '{
  "description": "One of the values the field lists in enum", "category": "basic",
  "schema": {"type": "string"},
  "normalize": ["trim"],
  "examples": ["active"]}'),
('reference', 'uuid', '{
  "description": "Id of an object of the type the field names in to", "category": "basic",
  "examples": ["0192e4a0-7b1c-7cc3-9d2e-3f4a5b6c7d8e"]}')
) AS seed (name, extends, def)
ON CONFLICT (name) DO NOTHING;
