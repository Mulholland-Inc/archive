/**
 * Archive runs your TypeScript against the `archive` SDK declared below, inside Archive.
 *
 * How a run works
 * - Your code is the body of an async function: use top-level await, and `return` a JSON value.
 * - Output: {"result": <returned value>, "logs": [...], "error": "" or a message}. console.log, warn and
 *   error add a line to logs; objects are written as JSON.
 * - Only `archive`, `console`, `fetch`, `input` and standard JavaScript exist: no require, import, timers, files or process.
 *   fetch(url, init?) calls other services (see Workflows); input is undefined unless a workflow trigger passed one.
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
 * - Each type is a table or a view in the public schema, and a row is a plain object (see Tables at the end).
 *   Views are read only: list, get and search them; create, update and delete go to their tables.
 *   Archive sets id, space, created_at and updated_at. Fields without ? are required on create; empty fields
 *   read back as null.
 * - Writes clean messy input before Postgres checks it, so pass what you have: "(415) 555-0123" becomes
 *   "+14155550123", "$1,234.50" becomes { amount: 1234.5, currency: "USD" }, "4/3/1990" becomes "1990-04-03".
 * - JSON values: date "2026-10-27"; timestamptz "2026-10-27T09:30:00Z" (UTC); numbers as numbers; uuid and
 *   references as id strings; arrays as lists; enums as one of their strings; the built-in composite types
 *   (Money, Address, ...) as the objects below. Secret types (Card, UsSsn, National, Passport)
 *   take the real value and read back masked; list and get with { reveal: true } show them if you are granted.
 *
 * Changing the schema (ontology admins)
 * - Types are the tables in the database, and nothing else is stored. archive.schema() generates their DDL from the
 *   database: one file per table or view, named <name>.sql, holding its CREATE TABLE, indexes, foreign keys, row-level
 *   security, policies, triggers, comments, and the enums, domains and functions only it uses (one used by no
 *   table gets its own <name>.sql). It is normalized: your layout, -- comments and statement order are not kept,
 *   COMMENT ON is. Change or add a file and pass the whole set to archive.apply(files): first without confirm to
 *   see the plan, then with { confirm: true }. A table in no file is dropped. apply(await archive.schema()) plans
 *   nothing.
 * - apply commits on its own as soon as it succeeds, even if the script throws later. It waits for locks on
 *   the tables and views it changes, including locks this run took by writing them or reading them with
 *   archive.sql (list, get and search hold no locks), so apply before those, or in a run of its own. After it,
 *   the new tables and views work in the same run.
 * - Every table needs id, space, created_at, updated_at and a search tsvector column, row-level security with
 *   policies, and should have COMMENT ON its table and columns: they become these docs. Use the built-in types
 *   (archive.email, archive.money, ...) to get their cleaning. A reference is `uuid REFERENCES other`; a list of
 *   references is `uuid[]` with a column comment starting 'references other'. Files load in any order, and a
 *   statement waits for the tables it needs; for tables that reference each other, add at least one of those
 *   references with ALTER TABLE ... ADD FOREIGN KEY after the CREATE TABLE. A complete file, house.sql:
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
 *     COMMENT ON TABLE house IS 'A house you own or rent
 *     icon: house';
 *     COMMENT ON COLUMN house.price IS 'Asking price';
 *
 * - Comments on tables and views: the first lines are the description; the last lines may be key: value
 *   settings. icon is a Lucide icon name in kebab-case (user, credit-card, square-check, ...; lucide.dev/icons),
 *   on tables and views. Views also take layout (table, board or calendar) and by (the column a board groups
 *   by, or the date a calendar places rows on). Any other key is an error.
 *
 * Views
 * - A view is a saved way to look at a table: a Postgres view in its own file, <view>.sql, with
 *   security_invoker = true so row-level security applies to whoever reads it, an id column, and a comment.
 *   A complete file, task_board.sql:
 *
 *     CREATE VIEW task_board WITH (security_invoker = true) AS
 *         SELECT * FROM task WHERE status <> 'done';
 *     COMMENT ON VIEW task_board IS 'Open tasks by progress
 *     layout: board
 *     by: status
 *     icon: kanban';
 *
 * - Create or change one: const files = await archive.schema(); files["task_board.sql"] = `...`;
 *   await archive.apply(files, { confirm: true }). Then archive.list("task_board") reads its rows, and
 *   archive.tables() shows its kind, description, layout, by and icon.
 * - Delete one: delete files["task_board.sql"] and apply. A view holds no data, so dropping it is never blocked.
 *
 * - A statement that deletes data (dropping a table or column, not a view) is blocked unless you pass { allowDrop: true }.
 *   A confirmed plan that deletes data runs right after this run commits, not during it, because the run holds
 *   locks on the tables it drops: apply returns the plan with deferred: true and applied: false, later calls in
 *   the same run still see the old tables, a run that throws drops nothing, and if the change fails the run's
 *   error says so.
 * - Renames are not detected: the plan would drop and re-add, losing data. Rename in the same call, and pass
 *   the files as they read after it: archive.apply(files, { confirm: true, renames:
 *   ["ALTER TABLE house RENAME COLUMN price TO asking_price"] }). The renames run first, in the same
 *   transaction, and are listed first in the plan, followed by your other changes.
 *
 * Workflows
 * - A workflow is a saved script, <name>.ts, that Archive runs by itself: on a schedule, when rows of a table
 *   are added or changed, when its webhook URL is called, or when started. It is written like the code you run
 *   now, and each of its runs is a run like this one: as you, one transaction, {timeout}, the same SDK, plus input.
 *   Every run's result, logs and error are kept in its history: archive.runs(name), and in the app, where the
 *   people you work for see their workflows (they never read the code).
 * - The first line of the file says when it runs, exactly in one of these forms:
 *     // on: cron 0 9 * * 1-5                         five fields, minute hour day month weekday, in UTC
 *     // on: cron CRON_TZ=America/New_York 0 9 * * *  the same in a time zone: 9:00 New York time, every day
 *     // on: cron @every 15m                          or @hourly, @daily, @weekly, @monthly
 *     // on: invoice                                  rows of the invoice table added or changed since its last run
 *     // on: webhook                                  every POST to its own URL
 *   Without an // on: line it runs only when started: archive.start(name), or the Run button in the app.
 * - input is what started the run:
 *     cron        undefined
 *     <table>     { rows: [...] }: up to 100 rows, as archive.list returns them, oldest change first; when there are
 *                 more, the next run follows at once. A row counts as changed when its created_at or updated_at
 *                 moves past the last run: archive.create and archive.update set them, raw SQL does not unless it
 *                 sets updated_at = now(). Rows the workflow writes itself never trigger it again.
 *     webhook     { method, query, headers, body }: query and headers are objects of strings (header names in lower
 *                 case); body is the parsed JSON when it parses, otherwise the raw text, and null when empty.
 *     start, test the input you pass, or undefined.
 * - archive.deploy(files) replaces all your workflows with files, the way apply does with the schema: start from
 *   archive.workflows(), change or add files, and pass the whole set; a file left out is deleted. It compiles each
 *   file and checks its name and // on: line, and if any is wrong it saves nothing and returns
 *   { deployed: false, errors: [{ file, message }] } instead of throwing. Otherwise it saves them at once, even if
 *   this run throws later, and returns { deployed: true, workflows: [{ name, on, next, url }] }: next is a cron
 *   workflow's next run, url a webhook's address. Deploying a file again keeps its webhook URL and its place in
 *   the table it watches.
 * - archive.test(nameOrSource, input) runs a saved workflow, or a draft's source, now, against the real data, and
 *   then undoes everything it wrote (except archive.apply and archive.deploy, which save at once). It returns
 *   { result, logs, error } like a run, error "" when it worked; compile errors come back there with their
 *   line:column. Test every workflow before you deploy it, with the input its trigger would pass, like
 *   { rows: await archive.list("invoice", { limit: 3 }) } for // on: invoice. The loop: write, test, read the
 *   error, fix, test again, deploy. A complete run that builds, tests and deploys two workflows:
 *
 *     const files = await archive.workflows();
 *     files["overdue-invoices.ts"] = `// on: cron CRON_TZ=America/New_York 0 9 * * 1-5
 *     const late = await archive.sql("SELECT id, number, due FROM invoice WHERE NOT paid AND due < current_date");
 *     console.log(late.length + " overdue");
 *     return late;`;
 *     files["add-invoice.ts"] = `// on: webhook
 *     const b = input.body;
 *     if (!b || !b.number) throw new Error("send JSON like { number, total, due }");
 *     const inv = await archive.create("invoice", { number: b.number, total: b.total, due: b.due });
 *     return { id: inv.id };`;
 *     const late = await archive.test(files["overdue-invoices.ts"]);
 *     const added = await archive.test(files["add-invoice.ts"], { method: "POST", query: {}, headers: {},
 *       body: { number: "INV-7", total: "$120.00", due: "2026-11-30" } });
 *     if (late.error || added.error) return { late, added };
 *     const done = await archive.deploy(files);
 *     if (!done.deployed) return done.errors;
 *     return { late: late.result, added: added.result, workflows: done.workflows };
 *
 *   The sources are template strings, so a backtick or ${ inside a workflow must be escaped as \` and \${.
 * - archive.start(name, input) runs a saved workflow for real inside this run and returns { result, logs, error };
 *   when it fails, only what it wrote is undone. Its run shows in archive.runs(name) once this run commits.
 * - Webhooks: anyone with the URL, ARCHIVE_URL/hooks/run/<secret>, can POST to it, with no other sign-in, so give
 *   it only to the service that calls it. The response is 200 with what the workflow returned, as JSON, or 500
 *   with { "error": message }. Other methods get 405 and an unknown URL 404.
 * - When a run fails (it throws, times out or returns too much), nothing it wrote is saved and its error is kept
 *   in the history. A cron workflow runs again at its next time; a table workflow gets the same rows again about
 *   a minute later, until a run works or you deploy a fix; a webhook caller gets the 500.
 * - fetch(url, init) calls any web address, in every run: init takes method, headers, and body as a string or as
 *   an object, which is sent as JSON. It returns { ok, status, statusText, url, headers, text(), json() } and
 *   throws when the address cannot be reached. It counts toward the run's {timeout}; responses over 5 MB fail.
 *   Nothing is undone outside Archive when the run fails: a request that was sent stays sent.
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
    /** Create an empty document named name, its type taken from the extension: .docx, .xlsx or .pptx; fill it with archive.edit (see Documents). */
    create(name: `${string}.${"docx" | "xlsx" | "pptx"}`, opts?: { space?: string }): Promise<StoredFile>;
    /** Change the given fields, null clears one, and return the row. */
    update<T extends keyof Tables>(table: T, id: string, patch: Partial<Tables[T]>): Promise<Tables[T]>;
    /** Delete a row. */
    delete(table: keyof Tables, id: string): Promise<void>;
    /** Full-text search over every table and file you can see, best first; words match by prefix, ignoring case and accents. */
    search(text: string, opts?: { type?: keyof Tables | "file"; space?: string; from?: string; to?: string; limit?: number }): Promise<Hit[]>;
    /** DDL generated from the database: <name>.sql to SQL, one file per table or view (see the header). */
    schema(): Promise<Record<string, string>>;
    /** Plan the whole set of DDL files against the database; with confirm, apply the plan (see the header). */
    apply(files: Record<string, string>, opts?: { confirm?: boolean; allowDrop?: boolean; renames?: string[] }): Promise<Plan>;
    /** Every table and view with its columns, types, comments and references, as Postgres has them now. */
    tables(): Promise<Table[]>;
    /** Files you can see, oldest first, with their extracted text; page with after = the last id. */
    files(opts?: { limit?: number; after?: string; space?: string }): Promise<StoredFile[]>;
    /** One file with its extracted text. */
    file(id: string): Promise<StoredFile>;
    /** The structure of a docx, xlsx or pptx file: paragraphs with ids, cells, slides and shapes (see Documents). */
    read(fileId: string): Promise<Model>;
    /** Apply edits to a docx, xlsx or pptx file and save it as the file's new content; all or nothing (see Documents). */
    edit(fileId: string, edits: Edit[]): Promise<StoredFile>;
    /** The spaces you can see, with your level in each. */
    spaces(): Promise<Space[]>;
    /** Your workflows: <name>.ts to its TypeScript (see Workflows in the header). */
    workflows(): Promise<Record<string, string>>;
    /** Replace all your workflows with files; a file left out is deleted. Returns errors instead of throwing, and saves nothing when there are any. */
    deploy(files: Record<string, string>): Promise<Deployed>;
    /** Run a saved workflow by name, or a draft's source, with input, then undo everything it wrote. */
    test(nameOrSource: string, input?: unknown): Promise<RunResult>;
    /** Run a saved workflow now, for real, inside this run, and record it in its history. */
    start(name: string, input?: unknown): Promise<RunResult>;
    /** A workflow's recent runs, newest first; limit defaults to 20, at most 100. */
    runs(name: string, opts?: { limit?: number }): Promise<WorkflowRun[]>;
};

/** What started this run: undefined for a plain run (see Workflows in the header). */
declare const input: any;

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

interface Deployed {
    deployed: boolean;
    /** When deployed is false: one per problem, and nothing was saved. */
    errors?: { file: string; message: string }[];
    workflows?: {
        name: string;
        /** The trigger from the // on: line; "" when it runs only when started. */
        on: string;
        /** A cron workflow's next run, UTC. */
        next?: string;
        /** A webhook workflow's URL. */
        url?: string;
    }[];
}

interface WorkflowRun {
    id: string;
    /** When it started, UTC. */
    at: string;
    /** What started it: cron, table, webhook, start (archive.start) or run (the Run button). */
    trigger: "cron" | "table" | "webhook" | "start" | "run";
    ms: number;
    result: any;
    logs: string[];
    /** "" when the run worked. */
    error: string;
}

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
    /** True when the plan deletes data and runs after this run commits. */
    deferred?: boolean;
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
