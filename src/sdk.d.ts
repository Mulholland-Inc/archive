/**
 * Archive runs your TypeScript against the `archive` SDK declared below, inside Archive.
 *
 * How a run works
 * - Your code is the body of an async function: use top-level await, and `return` a JSON value.
 * - Output: {"result": <returned value>, "logs": [...], "error": "" or a message}. console.log, warn and
 *   error add a line to logs; objects are written as JSON.
 * - Only `archive`, `console` and standard JavaScript exist: no fetch, require, import, timers, files or process.
 * - Limits: {timeout} per run, 256 KB of result JSON, 64 KB of logs. Return what you need, not whole tables.
 *
 * Transaction and access
 * - A run is one database transaction, as you. If it throws, times out or returns too much, everything rolls
 *   back and nothing is saved. A failed SDK call that you catch undoes only that call.
 * - Row-level security applies to everything, raw SQL included: you see and change only rows in your spaces.
 *   New rows go to your own space unless you pass { space: name }.
 * - SDK errors are Error objects with code ('invalid', 'not_found', 'forbidden', 'conflict' or a SQLSTATE)
 *   and, when invalid, errors: [{ path, message }] with one entry per bad field.
 *
 * Data
 * - Each type is a table in the public schema, and a row is a plain object (see Tables at the end).
 *   Archive sets id, space, created_at and updated_at. Fields without ? are required on create; empty fields
 *   read back as null.
 * - Writes clean messy input before Postgres checks it, so pass what you have: "06 12345678" becomes
 *   "+31612345678", "€ 1.234,50" becomes { amount: 1234.5, currency: "EUR" }, "3-4-1990" becomes "1990-04-03".
 * - JSON values: date "2026-10-27"; timestamptz "2026-10-27T09:30:00Z" (UTC); numbers as numbers; uuid and
 *   references as id strings; arrays as lists; enums as one of their strings; the built-in composite types
 *   (Money, Address, ...) as the objects below. Secret types (Card, UsSsn, NlBsn, National, Passport)
 *   take the real value and read back masked; list and get with { reveal: true } show them if you are granted.
 *
 * Changing the schema (ontology admins)
 * - Types are DDL files. Read them with archive.schema(), change or add a file, and pass the whole set to
 *   archive.apply(files): first without confirm to see the plan, then with { confirm: true }. A file you leave
 *   out is dropped, with its tables.
 * - apply commits on its own as soon as it succeeds, even if the script throws later. It waits for locks on
 *   the tables it changes, including locks this run took by reading or writing them, so apply before touching
 *   those tables, or in a run of its own. After it, the new tables work in the same run.
 * - Every table needs id, space, created_at, updated_at and a search tsvector column, row-level security with
 *   policies, and should have COMMENT ON its table and columns: they become these docs. Use the built-in types
 *   (archive.email, archive.money, ...) to get their cleaning. A reference is `uuid REFERENCES other`; a list of
 *   references is `uuid[]` with a column comment starting 'references other'. A complete file, house.sql:
 *
 *     CREATE TABLE house (
 *         id         uuid PRIMARY KEY DEFAULT uuidv7(),
 *         space      uuid NOT NULL REFERENCES archive.space,
 *         created_at timestamptz NOT NULL DEFAULT now(),
 *         updated_at timestamptz NOT NULL DEFAULT now(),
 *         address    text NOT NULL,
 *         price      archive.money,
 *         owner      uuid REFERENCES person,
 *         search     tsvector GENERATED ALWAYS AS (archive.search_doc(address)) STORED
 *     );
 *     CREATE INDEX house_search ON house USING gin (search);
 *     ALTER TABLE house ENABLE ROW LEVEL SECURITY;
 *     ALTER TABLE house FORCE ROW LEVEL SECURITY;
 *     CREATE POLICY viewer ON house FOR SELECT USING (space = ANY (archive.my_spaces()));
 *     CREATE POLICY editor ON house USING (space = ANY (archive.my_spaces('editor')))
 *         WITH CHECK (space = ANY (archive.my_spaces('editor')));
 *     COMMENT ON TABLE house IS 'A house you own or rent';
 *     COMMENT ON COLUMN house.price IS 'Asking price';
 *
 * - A statement that deletes data (dropping a table or column) is blocked unless you pass { allowDrop: true }.
 * - Renames are not detected: the plan would drop and re-add, losing data. Rename in the same call, and pass
 *   the files as they read after it: archive.apply(files, { confirm: true, renames:
 *   ["ALTER TABLE house RENAME COLUMN price TO asking_price"] }). The renames run first, in the same
 *   transaction, and are listed first in the plan, followed by your other changes.
 */
declare const archive: {
    /** Run one SQL statement as you, with $1, $2, ... bound to params; returns its rows, or [] when it has none. */
    sql(query: string, ...params: unknown[]): Promise<Row[]>;
    /** Rows of a table, oldest first; where matches exact (cleaned) values; page with after = the last id. */
    list<T extends keyof Tables>(table: T, opts?: { where?: Partial<Tables[T]>; limit?: number; after?: string; space?: string; reveal?: boolean }): Promise<Tables[T][]>;
    /** One row by id; throws not_found if it does not exist or you cannot see it. */
    get<T extends keyof Tables>(table: T, id: string, opts?: { reveal?: boolean }): Promise<Tables[T]>;
    /** Insert a row, cleaned and validated per field, and return it; it goes to opts.space or your own space. */
    create<T extends keyof Tables>(table: T, data: Partial<Tables[T]>, opts?: { space?: string }): Promise<Tables[T]>;
    /** Change the given fields, null clears one, and return the row. */
    update<T extends keyof Tables>(table: T, id: string, patch: Partial<Tables[T]>): Promise<Tables[T]>;
    /** Delete a row. */
    delete(table: keyof Tables, id: string): Promise<void>;
    /** Full-text search over every table and file you can see, best first; words match by prefix, ignoring case and accents. */
    search(text: string, opts?: { type?: keyof Tables | "file"; space?: string; from?: string; to?: string; limit?: number }): Promise<Hit[]>;
    /** The DDL files of your types: file name to SQL. */
    schema(): Promise<Record<string, string>>;
    /** Plan the DDL files against the database; with confirm, apply the plan and save the files (see the header). */
    apply(files: Record<string, string>, opts?: { confirm?: boolean; allowDrop?: boolean; renames?: string[] }): Promise<Plan>;
    /** Every table with its columns, types, comments and references, as Postgres has them now. */
    tables(): Promise<Table[]>;
    /** Files you can see, oldest first, with their extracted text; page with after = the last id. */
    files(opts?: { limit?: number; after?: string; space?: string }): Promise<StoredFile[]>;
    /** One file with its extracted text. */
    file(id: string): Promise<StoredFile>;
    /** The spaces you can see, with your level in each. */
    spaces(): Promise<Space[]>;
};

type Row = Record<string, any>;

/** Columns every table has; Archive sets them. */
interface Base {
    id: string;
    /** Id of the space the row is in. */
    space: string;
    created_at: string;
    updated_at: string;
}

interface Hit {
    kind: "entity" | "file";
    /** Table name, or "file". */
    type: string;
    id: string;
    space: string;
    date: string;
    /** The row, or the file's metadata. */
    data: Row;
    /** Matching text of a file, with the hits in **bold**. */
    snippet?: string;
    score: number;
}

interface Plan {
    statements: { ddl: string; hazards?: { type: string; message: string }[]; blocked?: boolean }[];
    applied: boolean;
}

interface Table {
    name: string;
    description: string;
    columns: {
        name: string;
        /** SQL type, like "text", "archive.money", "uuid[]" or "invoice_status". */
        type: string;
        description: string;
        /** Must be given on create. */
        required: boolean;
        /** Table this column references, like "public.person" or "archive.file". */
        references?: string;
        /** The allowed values of an enum. */
        values?: string[];
    }[];
}

interface StoredFile {
    id: string;
    name: string;
    mime: string;
    size: number;
    sha256: string;
    /** Extracted metadata: pages, dimensions, mail headers, ... */
    metadata: Row;
    /** pending, processing, done or error; text is ready when done. */
    status: string;
    error?: string;
    space: string;
    created_at: string;
    /** Text extracted from PDF, HTML, email and plain text; "" when none. */
    text: string;
}

interface Space {
    id: string;
    name: string;
    level: "owner" | "admin" | "editor" | "viewer";
}

/** Amount in major units with the currency's decimals: { amount: 1234.5, currency: "EUR" }. Also takes "€ 1.234,50" or a number. */
interface Money { amount: number; currency: Currency }
/** Instant in UTC with the IANA zone it was given in. Also takes "2026-10-27 09:30" or an RFC 3339 string. */
interface Timestamp { utc: string; zone: Timezone }
/** Time of day "09:30:00", with an optional zone. Also takes "9:30". */
interface Time { time: string; zone?: Timezone }
/** Start and end, each a date or UTC instant; null is open. */
interface Interval { start: string | null; end: string | null }
/** Postal address; country is taken from the locale when missing. */
interface Address { lines: string[]; locality?: string; region?: Region; postcode?: string; country?: Country }
/** Person name: the full original, and its parts when known. Also takes "Jan de Vries" or { given, family }. */
interface Name { full: string; given?: string; family?: string; prefixes?: string }
/** Postcode with the country it is valid in. Also takes "1012 LG". */
interface Postcode { country: Country; code: string }
/** WGS 84 coordinates. Also takes "52.37, 4.89". */
interface Geo { lat: number; lon: number }
/** Value in the SI base unit, with the UCUM unit it was given in. Also takes "5 km". */
interface Quantity { value: number; unit: string; original_unit: string }
