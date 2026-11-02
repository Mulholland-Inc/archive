/**
 * Archive runs your TypeScript against the `archive` SDK declared below, inside Archive.
 *
 * How a run works
 * - Your code is the body of an async function: use top-level await, and `return` a JSON value.
 * - Output: {"result": <returned value>, "logs": [...], "error": "" or a message}. console.log, warn and
 *   error add a line to logs; objects are written as JSON.
 * - Only `archive`, `console`, `fetch`, `input` and standard JavaScript exist: no require, import, timers, files or process.
 *   fetch(url, init?) calls other services (see Workflows); input is undefined unless a workflow's trigger passed one.
 * - Limits: {timeout} per run, 256 KB of result JSON, 64 KB of logs. Return what you need, not whole tables.
 *
 * Transaction and access
 * - A run is one database transaction, as you. If it throws, times out or returns too much, everything rolls
 *   back and nothing is saved. A failed SDK call that you catch undoes only that call.
 * - Your role in the workspace is viewer (reads), editor (also writes) or owner (also changes the schema and the
 *   members, sees every row and reveals secrets). Each row's access column says who sees it: null is the whole
 *   workspace, a group name like "hr" is only that group and the owners. Row-level security applies to
 *   everything, SQL included, so you see and change only the rows you may. A new row is for everyone unless
 *   you pass access: "hr" (a group you are in) or its table sets a default.
 * - People are rows of the member table: { email, role, groups }. Owners add, change and remove them with
 *   archive.create("member", { email: "ann@example.com", role: "editor", groups: ["hr"] }), archive.update and
 *   archive.delete; adding an email invites that person, and the last owner cannot be removed or demoted.
 * - SDK errors are Error objects with code ('invalid', 'not_found', 'forbidden', 'conflict' or a SQLSTATE)
 *   and, when invalid, errors: [{ path, message }] with one entry per bad field.
 *
 * Data
 * - Each type is a table or a view in the public schema, or one of Archive's own: member, workflow and
 *   workflow_run (see Workflows). A row is a plain object (see Tables, after this SDK).
 *   Views are read only: list, get and search them; create, update and delete go to their tables.
 *   Archive sets id, created_at and updated_at. Fields without ? are required on create; empty fields
 *   read back as null.
 * - Writes clean messy input before Postgres checks it, so pass what you have: "(415) 555-0123" becomes
 *   "+14155550123", "$1,234.50" becomes { amount: 1234.5, currency: "USD" }, "4/3/1990" becomes "1990-04-03".
 * - JSON values: date "2026-10-27"; timestamptz "2026-10-27T09:30:00Z" (UTC); numbers as numbers; uuid and
 *   references as id strings; arrays as lists; enums as one of their strings; the built-in composite types
 *   (Money, Address, ...) as the objects below. Secret types (Card, UsSsn, National, Passport)
 *   take the real value and read back masked; list and get with { reveal: true } show them to owners.
 * - archive.query(sql, ...params) reads with one SQL statement: joins, aggregates, anything list cannot. It only
 *   reads: a write or DDL in it throws forbidden. Rows change through archive.create, update and delete.
 *
 * Changing the schema (owners)
 * - Types are the tables in the database, and nothing else is stored. archive.schema() lists them with their
 *   columns and DDL, with indexes, foreign keys, triggers, policies and comments; archive.schema("house") is one:
 *     const { columns, ddl } = await archive.schema("house");
 * - archive.migrate(sql) dry-runs Postgres DDL: CREATE, ALTER or DROP of a TABLE, VIEW, INDEX, TYPE, DOMAIN,
 *   FUNCTION, TRIGGER, SEQUENCE or POLICY, and COMMENT ON. It runs the statements in order, all or nothing, undoes
 *   them, and returns a report: the statements with what Archive added; changes, each object they create, alter
 *   or drop with its definition before and after; drops, the changes that delete or rewrite data; and confirm.
 *   archive.migrate(sql, { confirm }) with that confirm runs it for real. The confirm holds while the schema and
 *   the sql stay the same; after any other migration it throws conflict, so dry-run again. Change rows with
 *   archive.create, update and delete, not in a migration.
 * - Write only your own columns. Archive adds to every table what you leave out: id uuid PRIMARY KEY DEFAULT
 *   uuidv7(), access text, created_at and updated_at timestamptz NOT NULL DEFAULT now(), a search column over its
 *   text columns with a gin index, and row-level security with the access policy; it adds id right after each
 *   CREATE TABLE, so a later REFERENCES or view can use it. When you add, drop or retype a text column, search
 *   follows. Write one yourself only to make it different: a table only the hr group sees has access text NOT
 *   NULL DEFAULT 'hr' CHECK (access = 'hr'). Use the built-in types (archive.email, archive.money, ...) to get
 *   their cleaning. A reference is `uuid REFERENCES other`; a list of references is `uuid[]` with a column
 *   comment starting 'references other'. Comment the table and its columns: they become these docs.
 *
 *     const sql = `
 *       CREATE TABLE house (
 *           address text NOT NULL,
 *           price   archive.money,
 *           owner   uuid REFERENCES person
 *       );
 *       COMMENT ON TABLE house IS 'A house you own or rent
 *       icon: house';
 *       COMMENT ON COLUMN house.price IS 'Asking price';`;
 *     const plan = await archive.migrate(sql);
 *     if (plan.drops.length > 0) return plan;
 *     await archive.migrate(sql, { confirm: plan.confirm });
 *
 * - Dropping a table or a column deletes its data, and changing a column's type rewrites it, so those are in
 *   drops. Confirm a migration with drops only when the people you work for asked for exactly those drops or
 *   agreed to them after you showed them: in chat, its confirm throws forbidden until a later turn, so dry-run
 *   it, tell them what it deletes, and stop. Drop nothing they did not ask for, like a column you copied from.
 * - A migration you confirm is part of this run: if the run throws or times out afterwards, it is undone with
 *   everything else. Later calls in the run see the change at once, so you can move data between steps:
 *     const migrate = async (sql) => {
 *       const plan = await archive.migrate(sql);
 *       if (plan.drops.length > 0) throw new Error("drops " + plan.drops.map((d) => d.name).join(", "));
 *       return archive.migrate(sql, { confirm: plan.confirm });
 *     };
 *     await migrate("ALTER TABLE house ADD COLUMN rooms integer");
 *     for (const h of await archive.list("house")) await archive.update("house", h.id, { rooms: 1 });
 *     await migrate("ALTER TABLE house ALTER COLUMN rooms SET NOT NULL");
 *   Rename with ALTER TABLE ... RENAME, and change a type with ALTER COLUMN ... TYPE ... USING: both keep the
 *   data. Postgres lets a value added to an enum be used only once the run that added it is saved, so add the
 *   value in one run and use it in the next. A migration waits up to 10s for locks on what it changes.
 * - Comments on tables and views: the first lines are the description; the last lines may be key: value
 *   settings. icon is a Lucide icon name in kebab-case (user, credit-card, square-check, ...; lucide.dev/icons),
 *   on tables and views. Views also take layout (table, board or calendar) and by (the column a board groups
 *   by, or the date a calendar places rows on). Any other key is an error.
 * - archive.revisions() lists the migrations saved before this run, newest first, with who ran them and their
 *   statements. To undo one, migrate the reverse.
 *
 * Views
 * - A view is a saved way to look at a table: a Postgres view with security_invoker = true so row-level security
 *   applies to whoever reads it, an id column, and a comment:
 *
 *     await archive.migrate(`
 *       CREATE VIEW task_board WITH (security_invoker = true) AS
 *           SELECT * FROM task WHERE status <> 'done';
 *       COMMENT ON VIEW task_board IS 'Open tasks by progress
 *       layout: board
 *       by: status
 *       icon: kanban';`);
 *
 * - Then archive.list("task_board") reads its rows, and archive.schema("task_board") shows its kind, description, layout, by
 *   and icon. Change one with CREATE OR REPLACE VIEW, delete it with DROP VIEW. Postgres saves SELECT * as the
 *   columns the table has then, so a view does not get columns added later, and a column a view selects cannot
 *   be dropped until you change or drop the view.
 *
 * Workflows
 * - A workflow is a saved script that Archive runs by itself: on a schedule, when rows of a table are added or
 *   changed, when its webhook URL is called, or when you run it. It is a row of the workflow table, which you
 *   write like any other: archive.create("workflow", { name, source, on }), archive.update("workflow", id, patch)
 *   and archive.delete("workflow", id), and read with list, get and query. source is written like the code you
 *   run now, and each of its runs is a run like this one: one transaction, {timeout}, the same SDK, plus input.
 *   It runs as whoever saved it last. You see and change only your own workflows; the people you work for see
 *   theirs in the app (they never read the code).
 * - on says when it runs:
 *     { cron: "0 9 * * 1-5" }                          five fields, minute hour day month weekday, in UTC
 *     { cron: "0 9 * * *", tz: "America/New_York" }    the same in a time zone: 9:00 New York time, every day
 *     { cron: "@every 15m" }                           or @hourly, @daily, @weekly, @monthly
 *     { table: "invoice", events: ["created", "updated"] }   rows of invoice added or changed since its last run;
 *                                                      events is created, updated or both (deletes never run it)
 *     { webhook: true }                                every POST to its own URL, the row's url
 *     null                                             only when run: archive.run(name, input), or the Run button
 * - input is what started the run:
 *     cron     { trigger: "cron", at }: at is when it started, UTC.
 *     table    { trigger: "table", table, created: [...], updated: [...] }: up to 100 rows in all, as archive.list
 *              returns them, oldest change first; when there are more, the next run follows at once. A row is
 *              created when archive.create added it since the last run, and updated when archive.update changed
 *              it since. Rows the workflow writes itself never run it again.
 *     webhook  { trigger: "webhook", method, query, headers, body }: query and headers are objects of strings
 *              (header names in lower case); body is the parsed JSON when it parses, otherwise the raw text, and
 *              null when empty.
 *     run      the input you pass archive.run, or undefined.
 * - Saving checks the workflow: source must compile, name is letters, digits, - and _, unique among yours, and on
 *   must be one of the forms above, its table one that exists and its cron and tz valid. A bad save throws
 *   invalid and saves nothing, with one entry in errors per problem: { path, message }, and line and column
 *   for a compile error in source. A save is part of this run, like any write: if the run throws later, it is
 *   undone. An edit or a rename keeps its webhook URL and its place in the table it watches; a cron workflow
 *   gets its next time from its new schedule when on changes.
 * - archive.run(name, input) runs a saved workflow now, for real, inside this run, and returns
 *   { result, logs, error }, error "" when it worked; when it fails, only what it wrote is undone. Its run is
 *   recorded in workflow_run once this run is saved. Each run's result, logs and error are kept there, the last
 *   100 of each workflow: list, get and query it, joined to workflow on workflow_run.workflow = workflow.id.
 * - The loop: write the workflow, save it with archive.create, read the errors of a refused save and fix them,
 *   run it with archive.run and the input its trigger would pass, read its result and error, fix it with
 *   archive.update, run it again. A run that saves and tries two workflows:
 *
 *     const save = async (name, source, on) => {
 *       const [old] = await archive.list("workflow", { where: { name } });
 *       return old ? archive.update("workflow", old.id, { source, on }) : archive.create("workflow", { name, source, on });
 *     };
 *     await save("overdue-invoices", `
 *       const late = await archive.query("SELECT id, number, due FROM invoice WHERE NOT paid AND due < current_date");
 *       console.log(late.length + " overdue");
 *       return late;`, { cron: "0 9 * * 1-5", tz: "America/New_York" });
 *     const hook = await save("add-invoice", `
 *       const b = input.body;
 *       if (!b || !b.number) throw new Error("send JSON like { number, total, due }");
 *       const inv = await archive.create("invoice", { number: b.number, total: b.total, due: b.due });
 *       return { id: inv.id };`, { webhook: true });
 *     const late = await archive.run("overdue-invoices", { trigger: "cron", at: new Date().toISOString() });
 *     const added = await archive.run("add-invoice", { trigger: "webhook", method: "POST", query: {}, headers: {},
 *       body: { number: "INV-7", total: "$120.00", due: "2026-11-30" } });
 *     return { late, added, url: hook.url };
 *
 *   A save that does not compile throws, with err.errors like [{ path: "source", line: 2, column: 11, message }].
 *   The sources are template strings, so a backtick or ${ inside a workflow must be escaped as \` and \${.
 *   archive.run runs it for real: added made invoice INV-7. To try a workflow without keeping what it writes,
 *   run it and then throw, which undoes the whole run.
 * - Webhooks: anyone with the URL, ARCHIVE_URL/hooks/run/<workspace>/<secret>, can POST to it, with no other
 *   sign-in, so give it only to the service that calls it. The response is 200 with what the workflow returned,
 *   as JSON, or 500 with { "error": message }. Other methods get 405 and an unknown URL 404.
 * - When a run fails (it throws, times out or returns too much), nothing it wrote is saved and its error is kept
 *   in workflow_run. A cron workflow runs again at its next time; a table workflow gets the same rows again about
 *   a minute later, until a run works or you save a fix; a webhook caller gets the 500.
 * - fetch(url, init) calls any web address, in every run: init takes method, headers, and body as a string or as
 *   an object, which is sent as JSON. It returns { ok, status, statusText, url, headers, text(), json() } and
 *   throws when the address cannot be reached. It counts toward the run's {timeout}; responses over 5 MB fail.
 *   Nothing is undone outside Archive when the run fails: a request that was sent stays sent.
 *
 * Browsing
 * - archive.browse(goal, { url }) has an agent use a real web browser for you: it opens url, or finds a site itself,
 *   then clicks, types and scrolls until goal is reached, and returns { done, answer, url }: done says whether it got
 *   there, answer is what it found or why it could not, and url is the page it ended on. Say what to find and how to
 *   answer: "Find the price of the cheapest plan; answer with just the amount". Use fetch when an API or a plain page
 *   has what you need; browse is for pages that need clicking, typing or JavaScript.
 * - Each call starts a fresh browser with no cookies, sign-ins or other state, and keeps nothing after it, so pages
 *   behind a sign-in are out of reach.
 * - A call takes seconds to minutes. A run browses for at most 5 minutes in all, and that time does not count
 *   toward its {timeout}.
 * - Browse before you write: browse throws once the run has written or migrated, so browse first, then save:
 *     const r = await archive.browse("Find the Saturday opening hours", { url: "https://example.com" });
 *     if (!r.done) throw new Error("no hours: " + r.answer);
 *     await archive.create("note", { text: r.answer, source: r.url });
 * - What it does on a site is not undone when the run fails: a form it sent stays sent.
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
    /** Create an empty document named name, its type taken from the extension: .docx, .xlsx or .pptx; fill it with archive.edit (see Documents). */
    create(name: `${string}.${"docx" | "xlsx" | "pptx"}`, opts?: { access?: string }): Promise<StoredFile>;
    /** Change the given fields, null clears one, and return the row. */
    update<T extends keyof Tables>(table: T, id: string, patch: Partial<Tables[T]>): Promise<Tables[T]>;
    /** Delete a row. */
    delete(table: keyof Tables, id: string): Promise<void>;
    /** Full-text search over every table and file you can see, best first; words match by prefix, ignoring case and accents. */
    search(text: string, opts?: { type?: keyof Tables | "file"; from?: string; to?: string; limit?: number }): Promise<Hit[]>;
    /** Every table and view with its columns, types, comments, references and DDL, as Postgres has them now. */
    schema(): Promise<Table[]>;
    /** One table or view; throws not_found, naming the ones there are, when it does not exist. */
    schema(name: keyof Tables): Promise<Table>;
    /** Dry-run DDL and report what it changes; with the report's confirm, run it as part of this run (see the header). */
    migrate(sql: string, opts?: { confirm?: string }): Promise<Migration>;
    /** Migrations saved before this run, newest first. */
    revisions(opts?: { limit?: number }): Promise<Revision[]>;
    /** Files you can see, oldest first, with their extracted text; page with after = the last id. */
    files(opts?: { limit?: number; after?: string }): Promise<StoredFile[]>;
    /** One file with its extracted text. */
    file(id: string): Promise<StoredFile>;
    /** The structure of a docx, xlsx or pptx file: paragraphs with ids, cells, slides and shapes (see Documents). */
    read(fileId: string): Promise<Model>;
    /** Apply edits to a docx, xlsx or pptx file and save it as the file's new content; all or nothing (see Documents). */
    edit(fileId: string, edits: Edit[]): Promise<StoredFile>;
    /** Run your saved workflow name now, for real, inside this run, and record it in workflow_run (see Workflows in the header). */
    run(name: string, input?: unknown): Promise<RunResult>;
    /** Have an agent use a fresh web browser to reach goal, starting at url; before any write, 5 minutes a run (see Browsing). */
    browse(goal: string, opts?: { url?: string }): Promise<BrowseResult>;
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
    /** created and updated together, oldest change first, for workflows written before they were split. */
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

interface Revision {
    id: string;
    /** When it ran, UTC. */
    at: string;
    /** The email of who ran it, and the token they used. */
    author: string;
    statements: string[];
}

interface Table {
    name: string;
    kind: "table" | "view";
    /** The comment without its key: value lines. */
    description: string;
    /** Lucide icon name from the comment's icon: line. */
    icon?: string;
    /** A view's layout: and by: lines. */
    layout?: "table" | "board" | "calendar";
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
    /** Its DDL, generated from the database: with its constraints, indexes, policies, triggers, comments, and the
     * types and functions only it uses. */
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
    /** pending, done or error; text is ready when done. */
    status: string;
    error?: string;
    /** Group that sees the file, null for the whole workspace. */
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
 * Office documents (docx, xlsx, pptx) are edited in place: an edit changes only
 * the XML it targets and every other part of the file is kept byte for byte, so
 * formatting, images, comments, charts and anything else survive.
 *
 * archive.create("Budget.xlsx") makes an empty one: a docx with one empty paragraph, an
 * xlsx with an empty Sheet1, a pptx with one title slide (shape 0 the title, shape 1 the subtitle).
 *
 * Workflow: read the model, then send a list of edits. Edits apply in order, all
 * or nothing: if one fails, nothing is saved and the error says which edit failed
 * and why. Call archive.read again after a successful edit before planning more.
 */

type Model = DocxModel | XlsxModel | PptxModel;

interface DocxModel {
  type: "docx";
  /** Every paragraph in document order, including those in tables and text boxes. */
  paragraphs: {
    id: string;        // "p0", "p1", ... Ids refer to the document as read; they stay valid
                       // for all edits in one archive.edit call, even after inserts and deletes.
    style?: string;    // paragraph style id, e.g. "Heading1"
    list?: string;     // "list 3 level 0" when the paragraph is a numbered or bulleted item
    in?: string;       // "t0/r1/c2" for a paragraph in table 0, row 1, cell 2; "textbox"
    text: string;      // tabs are "\t", line breaks "\n"
    runs?: string[];   // the text split by formatting runs, when there is more than one
  }[];
  /** Tables by index, as rows of cell text. Merged cells count once. */
  tables?: { id: string; rows: string[][] }[];
}

interface XlsxModel {
  type: "xlsx";
  sheets: {
    name: string;
    dimension?: string;                 // used range, e.g. "A1:H23"
    merged?: string[];                  // merged ranges, e.g. ["A1:G1"]
    tables?: { name: string; ref: string; columns: string[] }[];  // Excel tables
    cells: { ref: string; value?: string; formula?: string }[];   // non-empty cells; formulas without "="
  }[];
}

interface PptxModel {
  type: "pptx";
  slides: {
    slide: number;     // 1-based, in presentation order
    hidden?: boolean;  // hidden slides do not show in previews
    shapes: {          // shapes that hold text, by index
      shape: number;
      name?: string;
      placeholder?: string;  // "title", "ctrTitle", "subTitle", "body", "sldNum", ...
      paragraphs: { text: string; runs?: string[] }[];
    }[];
  }[];
}

type Edit =
  // ---- docx and pptx
  /**
   * Replace text inside one paragraph, even when it spans several formatting runs,
   * hyperlinks or tracked changes. The new text takes the formatting of the first
   * character replaced. Without "para" (docx) or "slide"/"shape" (pptx) the text must
   * occur in exactly one paragraph, unless "all" is true (then every occurrence is
   * replaced in docx, and the first occurrence in each paragraph in pptx). The text may not start or end with a tab or line break.
   * {op: "replace_text", find: "March 25th @ 10:30", replace: "March 26th @ 09:00"}
   * {op: "replace_text", para: "p2", find: "https://foo.com", replace: "the Foo homepage"}
   * {op: "replace_text", slide: 4, shape: 1, find: "Obama", replace: "Biden"}
   */
  | { op: "replace_text"; find: string; replace: string; para?: string; slide?: number; shape?: number; all?: boolean }

  // ---- docx
  /**
   * Insert a paragraph after another, formatted like it (same style, list numbering and
   * the font of its first run) unless "style" is given. "\n" in text becomes a line break.
   * Several inserts after the same id appear in the order given.
   * {op: "insert_paragraph_after", para: "p15", text: "Tika Workshop - March 27th"}
   */
  | { op: "insert_paragraph_after"; para: string; text: string; style?: string }
  /** {op: "delete_paragraph", para: "p9"}  Fails for the only paragraph in a cell, or one holding a section break. */
  | { op: "delete_paragraph"; para: string }
  /**
   * Replace all content of a table cell with one paragraph of text, keeping the cell's
   * paragraph and font formatting. table, row and col are 0-based indexes from the model.
   * {op: "set_cell_text", table: 0, row: 1, col: 2, text: "321"}
   */
  | { op: "set_cell_text"; table: number; row: number; col: number; text: string }

  // ---- xlsx ("sheet" defaults to the first sheet)
  /**
   * Set a cell to a constant. A JSON number stays a number, a string stays text (also
   * "123"), a boolean becomes TRUE/FALSE. The cell keeps its number format. Creates the
   * cell and row if needed. Formulas elsewhere recalculate when the file is opened.
   * {op: "set_cell", sheet: "Sheet1", cell: "C5", value: 4200}
   * {op: "set_cell", cell: "A5", value: "Transport"}
   */
  | { op: "set_cell"; cell: string; value: string | number | boolean; sheet?: string }
  /** {op: "set_formula", cell: "B6", formula: "SUM(B3:B5)"}  With or without "=". */
  | { op: "set_formula"; cell: string; formula: string; sheet?: string }
  /** {op: "clear_cell", cell: "D4"}  Empties the value and formula, keeps the format. */
  | { op: "clear_cell"; cell: string; sheet?: string }
  /**
   * Add a row directly below an Excel table and grow the table to include it, so totals,
   * formulas and charts that use the table pick it up. One value per column; a string
   * starting with "=" is a formula. The row below the table must be empty.
   * {op: "append_table_row", table_name: "tblExpenses", values: ["Internet", 60]}
   */
  | { op: "append_table_row"; table_name: string; values: (string | number | boolean)[]; sheet?: string }

  // ---- pptx
  /**
   * Replace all text of a shape. Each "\n" starts a new paragraph (a new bullet). The
   * text takes the paragraph and font formatting of the shape's first paragraph and run.
   * {op: "replace_shape_text", slide: 2, shape: 2, text: "Revenue up 12%\nCosts flat"}
   */
  | { op: "replace_shape_text"; slide: number; shape: number; text: string };

/**
 * Errors name the edit and say how to fix it, for example:
 *   edit 0 (replace_text): text "Boot Camp" occurs in 2 paragraphs (p14, p15); pass "para" or set "all": true
 *   edit 0 (replace_text): paragraph p3 does not contain "Mahout"; its text is "A title in ..."
 *   edit 0 (set_cell_text): row 9 does not exist; table 0 has 3 rows
 *   edit 0 (set_cell): B1 is hidden inside merged range A1:G1; write to its top-left cell A1 instead
 *   edit 0 (set_cell): C3 is part of the array formula over C3:C9, which cannot be edited cell by cell
 *   edit 0 (append_table_row): the row below table tblExpenses (row 24) is not empty (B24 = "x"); move that content first
 *   edit 1 (insert_paragraph_after): paragraph p9 was deleted or replaced by an earlier edit in this call
 *   edits must be a JSON array of objects with an "op" field: json: unknown field "cel"
 *
 * Not possible: inserting or deleting rows, columns, slides, shapes or tables; changing
 * formatting (bold, colors, number formats); comments; charts; headers and footers.
 */
