/**
 * Archive runs your TypeScript against the `archive` SDK below. Your code is the body of an async function: use
 * top-level await and `return` a JSON value. Output: { result, logs, error }; console.log, warn and error add to logs.
 * Only archive, console, fetch, input and standard JavaScript exist: no require, import, timers, files or process.
 * Limits: {timeout} a run, 256 KB of result, 64 KB of logs. Return what you need, not whole tables.
 *
 * Transaction and access
 * - A run is one transaction, as you. If it throws, times out or returns too much, everything rolls back. A failed
 *   SDK call you catch undoes only that call. What fetch, browse and actions did outside Archive stays done.
 * - viewer reads, editor also writes, owner also changes the schema and members, sees every row and reveals secrets.
 * - A row's access says who sees it: null is the workspace, a group is that group and the owners. Row-level security
 *   applies everywhere, SQL included. Writes take a group by id or name, like access: "hr"; reads give the id.
 * - Owners write the group table { name, description }; deleting one throws conflict while a row's access is it. Quote
 *   "group" in SQL. Owners write the member table { email, role, groups }: groups by id or name, a new name makes that
 *   group, adding an email invites them, and the last owner stays.
 * - SDK errors have code ('invalid', 'not_found', 'forbidden', 'conflict' or a SQLSTATE); invalid has errors: [{ path, message }].
 *
 * Data
 * - Tables, after this SDK, lists the tables and views. Views are read only. Archive sets id, created_at, updated_at.
 *   Set name on every create: it is the title every screen and link shows, and without it a row is Untitled. Fields
 *   without ? are required on create; empty fields read back null.
 * - Writes clean messy input, so pass what you have: "(415) 555-0123", "$1,234.50", "4/3/1990".
 * - JSON: date "2026-10-27"; timestamptz "2026-10-27T09:30:00Z"; uuids and references as id strings; enums as strings.
 *   Secret types (Card, UsSsn, National, Passport) read back masked; { reveal: true } shows them to owners.
 *
 * Schema (owners)
 * - archive.migrate(sql) dry-runs CREATE, ALTER or DROP of a TABLE, VIEW, INDEX, TYPE, DOMAIN, FUNCTION, TRIGGER,
 *   SEQUENCE or POLICY, and COMMENT ON, and returns a report with confirm. archive.migrate(sql, { confirm }) runs it as
 *   part of this run, and later calls see it. A confirm goes stale when the schema changes: that throws conflict, so dry-run
 *   again. Change rows with create, update and delete, not in a migration. Locks wait up to 10s.
 * - drops lists what deletes or rewrites data. Confirm those only when the people you work for asked for exactly them or
 *   agreed after you showed them. Talking with an agent, that confirm throws forbidden until a later turn: dry-run, tell
 *   them what it deletes, and stop.
 * - Keep data with ALTER TABLE ... RENAME and ALTER COLUMN ... TYPE ... USING. An enum value added is usable only from
 *   the next run. To undo a migration, migrate the reverse. Past ones:
 *   archive.query("SELECT created_at, author, statements FROM archive.revision WHERE statements <> '[]' ORDER BY created_at DESC LIMIT 20")
 * - Tables: CREATE TABLE house (address text NOT NULL, price archive.money, owner uuid REFERENCES person) INHERITS (archive.object);
 *   COMMENT ON TABLE house IS 'A house you own'; object gives id, name, access, created_at and updated_at, and Archive
 *   adds the primary key, a search column, and row-level security. Comments become these docs. Use the built-in archive.* types to
 *   get their cleaning. A list of references is uuid[] with a column comment starting 'references other'.
 * - Keep the title in name. To build it from columns: CREATE TRIGGER name_follows BEFORE INSERT OR UPDATE ON person FOR
 *   EACH ROW EXECUTE FUNCTION archive.name_follows('first_name', 'last_name'), or name_follows('full_name.full').
 * - A table only hr sees: access uuid NOT NULL DEFAULT '<hr's id>' CHECK (access = '<hr's id>').
 *
 * Views
 * - The app's sidebar lists views only, so to show rows there make a view, with id and name columns:
 *   CREATE VIEW task_board WITH (security_invoker = true) AS SELECT * FROM task WHERE status <> 'done';
 *   COMMENT ON VIEW task_board IS E'Open tasks\nlayout: board\nby: status';
 *   SELECT * keeps the table's columns as they are then. A column a view selects cannot be dropped until the view changes.
 * - The comment is a description, then key: value lines; only layout and by are keys. Layouts, with what by is:
 *   table; board (column to group by); calendar (date); list; map (archive.geo); gallery; chart (column, a date by month);
 *   timeline (date, to the view's next date column); tree (uuid to the view's own table, null at the root); stack (floor number, each
 *   row's width its first other number); scatter (number or money, up by the first other); year (date).
 *
 * Agents
 * - When they want a new agent, make it: archive.create("agent", { name, instructions, avatar, tint, access }), with
 *   instructions saying what it is for, a tint from 0 to 31 and an avatar that suits, by a number its column lists.
 * - Each person's conversation with an agent is theirs and never ends. turn holds it word for word, by n; memory sums it
 *   up, a row covering turns lo to lo+size-1 and its two halves of size/2 saying more. Both are read only. Search memory
 *   first, then read only the part of a turn you need, as a whole one can be very long:
 *   archive.query("SELECT lo, size, name FROM archive.memory WHERE agent = $1 AND name ~* $2 ORDER BY size, lo DESC LIMIT 20", id, word)
 *   archive.query("SELECT n, substring(messages::text, '(?i).{0,250}' || $2 || '.{0,250}') AS said FROM archive.turn WHERE agent = $1 AND messages::text ~* $2 ORDER BY n DESC LIMIT 5", id, word)
 *
 * Workflows
 * - A workflow is a saved script Archive runs by itself, a row of workflow: archive.create("workflow", { name, source, on, access }).
 *   source is written like this code and each run is a run like this one, as whoever saved it last. name is letters, digits,
 *   - and _. You see your own and those whose access is your group; only its owner changes or runs it.
 * - on: { cron: "0 9 * * 1-5", tz?: "America/New_York" } (UTC by default; also @every 15m, @hourly, @daily, @weekly, @monthly);
 *   { table: "invoice", events: ["created", "updated"] } (never deletes, nor rows it wrote itself); { webhook: true } (POST
 *   to the row's url, which anyone with it can call); null (only when run).
 * - input: cron { trigger, at }; table { trigger, table, created, updated, rows }, 100 rows at most with the next run
 *   following at once; webhook { trigger, method, query, headers, body }; or what you pass archive.run.
 * - A bad save throws invalid and saves nothing, with line and column for a compile error in source. Escape a backtick
 *   or ${ inside a template string. archive.run(name, input) runs it for real inside this run; { error } says if it
 *   failed, undoing only what it wrote. To try one without keeping what it writes, throw afterwards. So: save, run,
 *   read the result, archive.update, run again.
 * - workflow_run keeps the last 100 runs of each, joined on workflow_run.workflow = workflow.id. After a failure a cron
 *   waits for its next time, a table workflow retries the same rows about a minute later, and a webhook caller gets 500.
 *   Its response is otherwise 200 with what it returned.
 * - fetch counts toward the run's {timeout}; responses over 5 MB fail.
 *
 * Files
 * - Folders: archive.create("file", { kind: "folder", name, parent, access }). Move with archive.update("file", id, { parent }), null for the top.
 *   Bytes held, each once: archive.query("SELECT archive.stored()").
 *
 * Browsing
 * - Each archive.browse starts a fresh browser with no sign-ins, so pages behind one are out of reach. Use fetch when an API
 *   or a plain page has it. Say what to find and how to answer. It throws once the run has written, so browse first, then save.
 *
 * Models
 * - archive.model: model is a gemini- or claude- name, like "gemini-3.8-flash" or "claude-haiku-5-5"; a model whose key the server
 *   lacks throws. files are from archive.files, up to 20 MB each and 10 a call. With schema, json is the parsed answer.
 *   effort "none" turns thinking off. Each call has 2 minutes. The transaction stays open meanwhile, so read with models
 *   first and write after when you can.
 * - archive.do hands goal to a one-off agent with no memory, which runs scripts until done. They join this run's
 *   transaction. maxRuns is 25 by default, 50 at most. Say what to do and how to answer.
 * - A run browses for 5 minutes at most, calls models for 10 and runs agents for 10, none of it counting toward {timeout}.
 *
 * Connected accounts
 * - People connect their own Gmail, Outlook, Drive or Notion in Settings, Connectors; owners connect integrations for
 *   everyone in Archive settings, Integrations, and may narrow one to a group. Both are rows of connection. To use an
 *   account, a person connects it there: you cannot.
 * - archive.action("gmail.search", { query }) uses one; Actions lists yours. With several accounts at a connector, pass
 *   { connection }, its account or id. Integrations act for editors and owners. Failures throw, with code unauthorized when
 *   the account must connect again. An action counts toward {timeout}. In a workflow, actions are its runner's.
 * - What connected accounts have is also synced into tables, for this workspace's connectors only, so read those first, without calling
 *   the service. Rows have connection, stream and source_id; Streams says which stream lands where. A personal account's rows only its
 *   owner sees. connection.synced_at says when one last caught up; disconnecting deletes what it synced.
 * - A synced file is read only here: read it with archive.file and archive.read, archive.copy it to change it, or send one
 *   of yours there with its connector's upload: archive.action("gdrive.upload", { file, folder, replace }), by source_id.
 */
declare const archive: {
    /** Read with one SQL statement as you, with $1, $2, ... bound to params; returns its rows, or [] when it has none. Writes and DDL throw forbidden. */
    query(sql: string, ...params: unknown[]): Promise<Row[]>;
    /** Rows of a table, oldest first; where matches exact (cleaned) values; page with after = the last id. */
    list<T extends keyof Tables>(table: T, opts?: { where?: Partial<Tables[T]>; limit?: number; after?: string; reveal?: boolean }): Promise<Tables[T][]>;
    /** One row by id; throws not_found if it does not exist or you cannot see it. */
    get<T extends keyof Tables>(table: T, id: string, opts?: { reveal?: boolean }): Promise<Tables[T]>;
    /** Insert a row, cleaned and validated per field, and return it; data.access says which group sees it, null for everyone. */
    create<T extends keyof Tables>(table: T, data: Partial<Tables[T]>): Promise<Tables[T]>;
    /** Create an empty document named name, its type taken from the extension; fill it with archive.edit (see Documents). */
    create(name: `${string}.${"docx" | "xlsx" | "pptx"}`, opts?: { access?: string }): Promise<StoredFile>;
    /** Change the given fields, null clears one, and return the row. */
    update<T extends keyof Tables>(table: T, id: string, patch: Partial<Tables[T]>): Promise<Tables[T]>;
    /** Delete a row, or with "file" a file or empty folder of yours; what a connection synced goes when it goes there. */
    delete(table: keyof Tables, id: string): Promise<void>;
    /** Full-text search over every table and file you can see, best first; words match by prefix, ignoring case and accents. */
    search(text: string, opts?: { type?: keyof Tables | "file"; from?: string; to?: string; limit?: number }): Promise<Hit[]>;
    /** Your tables and views, with their columns, comments, references and DDL, as Postgres has them now. */
    schema(): Promise<Table[]>;
    /** Any table or view, Archive's own too, and those of connectors this workspace has not connected; throws not_found, naming the ones there are. */
    schema(name: string): Promise<Table>;
    /** Dry-run DDL and report what it changes; with the report's confirm, run it as part of this run (see Schema). */
    migrate(sql: string, opts?: { confirm?: string }): Promise<Migration>;
    /** Files and folders you can see, yours and what your connections synced, oldest first, with their extracted text; page with after = the last id. */
    files(opts?: { limit?: number; after?: string }): Promise<StoredFile[]>;
    /** One file with its extracted text. */
    file(id: string): Promise<StoredFile>;
    /** A file of yours, to change with archive.edit, from any file you can read: the same bytes and words. A file of your personal connection needs opts.access, a group's name or "" for everyone, as nothing else says who sees the copy. */
    copy(id: string, opts?: { name?: string; access?: string }): Promise<StoredFile>;
    /** The structure of a docx, xlsx or pptx file: paragraphs with ids, cells, slides and shapes (see Documents). */
    read(fileId: string): Promise<Model>;
    /** Apply edits to a docx, xlsx or pptx file of yours and save it as the file's new content; all or nothing (see Documents). What a connection synced is read only: archive.copy it first. */
    edit(fileId: string, edits: Edit[]): Promise<StoredFile>;
    /** Run your saved workflow name now, for real, inside this run, and record it in workflow_run (see Workflows in the header). */
    run(name: string, input?: unknown): Promise<RunResult>;
    /** Have an agent use a fresh web browser to reach goal, starting at url; before any write, 5 minutes a run (see Browsing). */
    browse(goal: string, opts?: { url?: string }): Promise<BrowseResult>;
    /** Ask a gemini- or claude- model once, with files (a PDF page with page) and a JSON schema for json; 10 minutes a run (see Models). */
    model(req: ModelReq): Promise<ModelResult>;
    /** Ask several models at once, eight at a time; the results come in the order of reqs and the first error fails them all (see Models). */
    model(reqs: ModelReq[]): Promise<ModelResult[]>;
    /** Have an agent reach goal by writing and running scripts inside this run's transaction, and return its answer; 10 minutes a run (see Models). */
    do(goal: string, opts?: { model?: string; instructions?: string; files?: { id: string; page?: number }[]; maxRuns?: number }): Promise<DoResult>;
    /** Do the action name, like "gmail.search", with one of your connected accounts; connection picks one when you have several (see Connected accounts). */
    action<A extends keyof Actions>(name: A, input: Actions[A]["input"], opts?: { connection?: string }): Promise<Actions[A]["output"]>;
};

/** What started this run (see Workflows in the header): undefined for a plain run, and what was passed for archive.run. */
declare const input: CronInput | TableInput | WebhookInput | undefined;

/** When a workflow runs: exactly one of cron, table or webhook; null on the row for one that only runs when run. */
type WorkflowOn =
    | { cron: string; /** IANA time zone; UTC when left out. */ tz?: string }
    | { table: keyof Tables; events: ("created" | "updated")[] }
    | { webhook: true };

interface CronInput {
    trigger: "cron";
    /** When it started, UTC. */
    at: string;
}

interface TableInput {
    trigger: "table";
    table: string;
    /** Rows added since the last run, oldest first. */
    created: Row[];
    /** Rows changed since the last run, oldest first; up to 100 rows with created. */
    updated: Row[];
    /** created and updated together, oldest change first. */
    rows: Row[];
}

interface WebhookInput {
    trigger: "webhook";
    method: string;
    query: Record<string, string>;
    /** Header names in lower case. */
    headers: Record<string, string>;
    /** Parsed JSON, the raw text when it is not JSON, or null when empty. */
    body: any;
}

/** Call a web address; body is a string, or an object sent as JSON. Throws when it cannot be reached. */
declare function fetch(url: string, init?: { method?: string; headers?: Record<string, string>; body?: string | object }): Promise<FetchResponse>;

interface FetchResponse {
    ok: boolean;
    status: number;
    statusText: string;
    url: string;
    /** Header names in lower case; headers.get(name) works too. */
    headers: Record<string, string> & { get(name: string): string | null };
    text(): Promise<string>;
    /** Throws when the body is not JSON. */
    json(): Promise<any>;
}

interface RunResult {
    result: any;
    logs: string[];
    /** "" when the run worked. */
    error: string;
}

interface BrowseResult {
    /** Whether it reached the goal. */
    done: boolean;
    /** What it found, as the goal asked; or why it could not. */
    answer: string;
    /** The page it ended on. */
    url: string;
}

interface ModelReq {
    model: string;
    prompt: string;
    system?: string;
    files?: { id: string; page?: number }[];
    schema?: object;
    effort?: "none";
}

interface ModelResult {
    /** The answer as text; with a schema, the JSON as a string. */
    text: string;
    /** The answer parsed, when a schema was given. */
    json?: any;
    tokens_in: number;
    tokens_out: number;
}

interface DoResult {
    /** The agent's final answer, as the goal asked. */
    answer: string;
    /** How many scripts it ran. */
    runs: number;
    tokens_in: number;
    tokens_out: number;
}

type Row = Record<string, any>;

/** Columns every table has; Archive sets them. */
interface Base {
    id: string;
    created_at: string;
    updated_at: string;
}

interface Hit {
    kind: "entity" | "file";
    /** Table name, or "file". */
    type: string;
    id: string;
    date: string;
    /** The row, or the file's metadata. */
    data: Row;
    /** Matching text of a file, with the hits in **bold**. */
    snippet?: string;
    score: number;
}

interface Migration {
    /** false for a dry run, which changed nothing. */
    applied: boolean;
    /** Pass it as { confirm } to run this migration for real. */
    confirm: string;
    /** The revision it saved, once the run is; only when applied. */
    revision?: string;
    /** Your statements, then what Archive added, in the order they ran. */
    statements: string[];
    /** What they create, alter or drop; a table created or dropped whole is one change, with its DDL. */
    changes: SchemaChange[];
    /** The changes that delete or rewrite data: dropped tables, columns and type attributes, and columns whose type changed. */
    drops: SchemaChange[];
}

interface SchemaChange {
    kind: "table" | "column" | "constraint" | "index" | "policy" | "trigger" | "view" | "type" | "attribute" | "domain" | "function" | "sequence";
    /** Like house, house.rooms or touch(). */
    name: string;
    action: "create" | "alter" | "drop";
    /** The name it had, when renamed. */
    from?: string;
    /** Its definition as Postgres renders it. */
    before?: string;
    after?: string;
}

interface Table {
    name: string;
    kind: "table" | "view";
    /** The comment without its key: value lines. */
    description: string;
    /** A view's layout: and by: lines. */
    layout?: "table" | "board" | "calendar" | "list" | "map" | "gallery" | "chart" | "timeline" | "tree" | "stack"
        | "scatter" | "year";
    by?: string;
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
    /** Its DDL with its constraints, indexes, policies, triggers and comments; "" for Archive's own tables. */
    ddl: string;
}

interface StoredFile {
    id: string;
    name: string;
    mime: string;
    size: number;
    sha256: string;
    /** Extracted metadata: pages, dimensions, mail headers, ... */
    metadata: Row;
    /** pending, done, skipped (Archive does not copy it, as error says) or error; text is ready when done. */
    status: string;
    error?: string;
    /** file or folder. */
    kind: string;
    /** The id of the folder it is in, null at the top. */
    parent: string | null;
    /** The connection it is synced from, null for one of yours; a connection's is read only. */
    connection: string | null;
    /** Its id there, for a connector's upload { folder, replace }. */
    source_id?: string;
    /** The token there of its content. */
    version?: string;
    /** Where it opens there. */
    url?: string;
    /** The id of the group that sees the file, null for the whole workspace. */
    access: string | null;
    created_at: string;
    /** Text extracted from PDF, HTML, email and plain text; "" when none. */
    text: string;
}

/** Amount in major units with the currency's decimals: { amount: 1234.5, currency: "USD" }. Also takes "$1,234.50" or a number. */
interface Money { amount: number; currency: Currency }
/** Instant in UTC with the IANA zone it was given in. Also takes "2026-10-27 09:30" or an RFC 3339 string. */
interface Timestamp { utc: string; zone: Timezone }
/** Time of day "09:30:00", with an optional zone. Also takes "9:30". */
interface Time { time: string; zone?: Timezone }
/** Start and end, each a date or UTC instant; null is open. */
interface Interval { start: string | null; end: string | null }
/** Postal address; country is taken from the locale when missing. */
interface Address { lines: string[]; locality?: string; region?: Region; postcode?: string; country?: Country }
/** Person name: the full original, and its parts when known. Also takes "Mary Ann Smith" or { given, family }. */
interface Name { full: string; given?: string; family?: string; prefixes?: string }
/** Postcode with the country it is valid in. Also takes "94105". */
interface Postcode { country: Country; code: string }
/** WGS 84 coordinates. Also takes "40.71, -74.01". */
interface Geo { lat: number; lon: number }
/** Value in the SI base unit, with the UCUM unit it was given in. Also takes "5 km". */
interface Quantity { value: number; unit: string; original_unit: string }

/**
 * Documents
 * docx, xlsx and pptx files are edited in place: everything an edit does not target survives byte for byte.
 * archive.create("Budget.xlsx") makes an empty one: a docx with one empty paragraph, an xlsx with an empty Sheet1, a
 * pptx with one title slide (shape 0 the title, shape 1 the subtitle).
 * archive.read the model, then send edits. They apply in order, all or nothing, and an error says which failed and how
 * to fix it. Read again after a successful edit before planning more.
 * Not possible: inserting or deleting rows, columns, slides, shapes or tables; formatting; comments; charts; headers and footers.
 */

type Model = DocxModel | XlsxModel | PptxModel;

interface DocxModel {
  type: "docx";
  /** Every paragraph in document order, including those in tables and text boxes. */
  paragraphs: {
    id: string;        // "p0", "p1", ...: valid for all edits in one archive.edit call, even after inserts and deletes
    style?: string;    // like "Heading1"
    list?: string;     // "list 3 level 0" for a numbered or bulleted item
    in?: string;       // "t0/r1/c2" for table 0, row 1, cell 2; "textbox"
    text: string;      // tabs are "\t", line breaks "\n"
    runs?: string[];   // the text by formatting run, when there is more than one
  }[];
  /** Merged cells count once. */
  tables?: { id: string; rows: string[][] }[];
}

interface XlsxModel {
  type: "xlsx";
  sheets: {
    name: string;
    dimension?: string;                 // like "A1:H23"
    merged?: string[];                  // like ["A1:G1"]
    tables?: { name: string; ref: string; columns: string[] }[];  // Excel tables
    cells: { ref: string; value?: string; formula?: string }[];   // non-empty cells; formulas without "="
  }[];
}

interface PptxModel {
  type: "pptx";
  slides: {
    slide: number;     // 1-based
    hidden?: boolean;
    shapes: {          // shapes that hold text, by index
      shape: number;
      name?: string;
      placeholder?: string;  // "title", "ctrTitle", "subTitle", "body", "sldNum", ...
      paragraphs: { text: string; runs?: string[] }[];
    }[];
  }[];
}

type Edit =
  // docx and pptx. Spans formatting runs and takes the first replaced character's formatting. Without para (docx) or
  // slide and shape (pptx) the text must be in exactly one paragraph, unless all: every occurrence in docx, the first in
  // each paragraph in pptx. It may not start or end with a tab or line break.
  | { op: "replace_text"; find: string; replace: string; para?: string; slide?: number; shape?: number; all?: boolean }
  // docx. Formatted like para unless style is given; "\n" is a line break; several after one id keep their order.
  | { op: "insert_paragraph_after"; para: string; text: string; style?: string }
  // docx. Fails for the only paragraph in a cell, or one holding a section break.
  | { op: "delete_paragraph"; para: string }
  // docx. 0-based indexes; replaces the cell with one paragraph, keeping its formatting.
  | { op: "set_cell_text"; table: number; row: number; col: number; text: string }
  // xlsx; sheet defaults to the first. A number stays a number, a string text (also "123"), keeping the cell's format.
  | { op: "set_cell"; cell: string; value: string | number | boolean; sheet?: string }
  // xlsx. With or without "=".
  | { op: "set_formula"; cell: string; formula: string; sheet?: string }
  // xlsx. Empties value and formula, keeps the format.
  | { op: "clear_cell"; cell: string; sheet?: string }
  // xlsx. Adds a row directly below an Excel table and grows the table, one value per column; "=" starts a formula. The row below must be empty.
  | { op: "append_table_row"; table_name: string; values: (string | number | boolean)[]; sheet?: string }
  // pptx. Each "\n" starts a paragraph, formatted like the shape's first.
  | { op: "replace_shape_text"; slide: number; shape: number; text: string };
