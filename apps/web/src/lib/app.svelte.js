// The app's state. An archive is one Archive workspace: its chats, files, tables, members and workflows come from
// the API (see live.js for the mapping). What the API cannot do yet comes from mock/ and stays in this tab or
// in localStorage; mock/index.js lists those pieces.

import { API, api, blobURL, forgetBlobs, upload, setWorkspace, workspace, local, setLocal, signOut as apiSignOut, Unauthorized, passkeys, passkeyAsked } from './api.js';
import { resource, listen, onChange, onRefused } from './stream.svelte.js';
import { toFile, toChat, toMember, toWorkflow, toRun, relative, show } from './live.js';
import { toTables, countsQuery } from './schema.js';
import { initials, mockConnectors, mockKey, mockProfile, mockSettings } from './mock/index.js';
import { catalog } from './mock/connectors.js';
import { lookOf } from './looks.js';
import { notify } from './dialogs.svelte.js';
import { go, replace, route } from './router.svelte.js';

let counter = 0;
const nid = (prefix) => `${prefix}${Date.now().toString(36)}${(counter++).toString(36)}`;

export const app = $state({
  status: 'loading',
  user: { id: '', name: '', first: '', email: '', plan: '', tone: 1 },
  role: 'viewer',
  canAdd: false,
  askPasskey: false,
  archives: [],
  archive: null,
  collapsed: typeof localStorage !== 'undefined' && localStorage.getItem('archive.sidebar') === 'collapsed',
  drawer: false,
  details: false,
  palette: false,
  help: false,
  model: 'pro',
  pending: null,
  muted: false,
  settings: { open: false, section: null, sub: null },
  archiveSettings: { open: false, section: null, sub: null },
  apiKey: mockKey('ak_live_', 'Created today'),
  archiveKeys: {},
  archiveApps: {},
  share: { open: false, kind: null, target: null },
  onboarding: [],
  profile: mockProfile()
});

const kindOf = (type) => ({ family: 'Family', business: 'Business', personal: 'Just you' })[type] ?? 'Archive';

const lookBody = (look) => ({ type: look.type, icon: look.icon, color: look.tint });

function shell(w) {
  const look = lookOf(w);
  return {
    id: w.id,
    name: w.name,
    kind: kindOf(look.type),
    tone: (w.id.charCodeAt(w.id.length - 1) % 6) + 1,
    look,
    settings: mockSettings(),
    chats: [],
    conversations: {},
    library: [],
    folders: [],
    workflows: [],
    members: [],
    groups: [],
    usage: {},
    schema: [],
    archiveTables: [],
    names: {},
    shares: [],
    connectors: mockConnectors(w.id),
    catalog: null,
    counts: null,
    loading: true
  };
}

export function ws() {
  return app.archives.find((w) => w.id === app.archive) ?? app.archives[0];
}

const call = (path, opts = {}) => api(path, { ...opts, workspace: opts.workspace ?? app.archive });

async function complain(error, title = 'That didn’t work') {
  if (error instanceof Unauthorized) {
    app.status = 'signedOut';
    return;
  }
  await notify({ title, body: error?.message ?? String(error) });
}

// Signing in and loading

export async function boot() {
  const query = new URLSearchParams(location.search);
  const named = query.get('workspace');
  // A connector's consent sends the browser back here, saying why it did not connect when it did not.
  if (query.get('error')) notify({ title: 'That didn’t connect', body: query.get('error') });
  if (named) {
    setWorkspace(named);
    history.replaceState(null, '', location.pathname);
  }
  let me;
  try {
    me = await api('/auth/me');
  } catch (e) {
    // The archive this device opened last may be gone; open your first one instead.
    if (!(e instanceof Unauthorized) && workspace() && e.status >= 403) {
      setWorkspace('');
      return boot();
    }
    if (!(e instanceof Unauthorized)) throw e;
    app.status = 'signedOut';
    return;
  }
  setMe(me);
  setWorkspace(me.workspace);
  const list = me.workspaces?.length ? me.workspaces : [{ id: me.workspace, name: 'Archive', type: 'personal' }];
  app.archives = list.map(shell);
  app.archive = me.workspace;
  app.onboarding = local(`archive.onboarding:${me.workspace}`, []);
  app.askPasskey = passkeys && !passkeyAsked(me.actor.user_id);
  app.status = 'ready';
  listen(app.archive);
  account.seen();
  loadArchive(ws());
}

// Who you are and which archives you have, read again when your account, an archive's name or your role moves,
// in this tab or anywhere else. An archive you were removed from, or that was deleted, goes; one you were
// added to arrives.
async function loadMe() {
  if (app.status !== 'ready') return;
  let me;
  let here = true;
  try {
    me = await api('/auth/me', { workspace: app.archive });
  } catch (e) {
    if (e instanceof Unauthorized) return leave();
    // This archive cannot be read: you may have left it, or it was deleted. Ask who you are without it.
    here = false;
    setWorkspace('');
    me = await api('/auth/me').catch((e) => (e instanceof Unauthorized ? leave() : null));
    if (!me) return setWorkspace(app.archive);
  }
  if (here) setMe(me);
  const list = me.workspaces?.length ? me.workspaces : [{ id: me.workspace, name: 'Archive', type: 'personal' }];
  const had = new Map(app.archives.map((v) => [v.id, v]));
  app.archives = list.map((w) => {
    const v = had.get(w.id) ?? shell(w);
    Object.assign(v, { name: w.name, look: lookOf(w), kind: kindOf(w.type) });
    return v;
  });
  if (!app.archives.some((v) => v.id === app.archive)) {
    app.archive = null;
    await switchArchive(me.workspace);
    go('/');
  } else if (!here) setWorkspace(app.archive);
  app.canAdd = !!me.admin;
}

function leave() {
  listen('');
  app.status = 'signedOut';
}

export const account = resource({
  tables: (c) => c.table === 'archive.workspace' || c.table === 'archive.member' || (c.table === 'archive.user' && (!c.id || c.id === app.user.id)),
  load: loadMe
});

// A socket the archive refuses may mean you left it or it is gone.
onRefused(() => app.status === 'ready' && account.refresh());

// Another tab of this browser signing out signs this one out too.
const tabs = typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('archive') : null;
tabs?.addEventListener('message', (e) => {
  if (e.data === 'signedOut' && app.status === 'ready') {
    leave();
    app.archives = [];
    app.archive = null;
    go('/');
  }
});

function setMe(me) {
  const name = me.actor.name || me.actor.email;
  app.user = { id: me.actor.user_id, name, first: name.split(/[\s@]+/)[0], email: me.actor.email, plan: me.role.charAt(0).toUpperCase() + me.role.slice(1), tone: 1 };
  app.role = me.role;
  // Admins of this Archive make and delete archives, whichever archive they are in.
  app.canAdd = !!me.admin;
  Object.assign(app.profile, { name, first: app.user.first });
}

async function files(id) {
  const all = [];
  for (let after = ''; ; ) {
    const next = await api(`/api/files?limit=500${after ? `&after=${after}` : ''}`, { workspace: id });
    all.push(...next);
    if (next.length < 500) break;
    after = next.at(-1).id;
  }
  return all.reverse();
}

// What an archive shows, each read again when what it is read from moves (stream.svelte.js). Each reads into
// the archive that is open when it starts.

// The chats, newest first. A chat being written here and not saved yet stays at the top, and what this tab
// knows of a chat (it is streaming, its messages are loaded) stays with it.
export const chats = resource({
  tables: ['archive.chat'],
  load: async () => {
    const v = ws();
    const list = (await api('/api/chats', { workspace: v.id })) ?? [];
    const pins = new Set(local(`archive.pins:${v.id}`, []));
    const had = new Map(v.chats.map((c) => [c.id, c]));
    const next = list.map((c) => {
      const old = had.get(c.id);
      const fresh = toChat(c, pins.has(c.id));
      return old ? Object.assign(old, fresh, { updated: old.streaming ? old.updated : fresh.updated }) : fresh;
    });
    const ids = new Set(next.map((c) => c.id));
    v.chats = [...v.chats.filter((c) => c.draft || (c.streaming && !ids.has(c.id))), ...next];
    for (const id of Object.keys(v.conversations)) {
      if (!ids.has(id) && !v.chats.some((c) => c.id === id)) delete v.conversations[id];
    }
  }
});

// The library's files, and its folders: the name of every group a file is limited to, and the empty ones made
// here.
export const library = resource({
  tables: ['archive.file', 'archive.group'],
  load: async () => {
    const v = ws();
    const [list] = await Promise.all([files(v.id), groups.refresh()]);
    v.library = list.map((f) => filed(toFile(f, v.name), v));
    v.folders = [...new Set([...v.library.map((f) => f.folder), ...v.folders])];
  }
});

// A file's bytes and thumbnail are cached as blobs; a file that moved has them fetched again.
onChange((changes) => {
  for (const c of changes) if (c.table === 'archive.file') forgetBlobs(c.id ? `/api/files/${c.id}/` : '/api/files/');
});

export const members = resource({
  tables: ['archive.member', 'archive.user'],
  load: async () => {
    const v = ws();
    const list = (await api('/auth/users', { workspace: v.id })) ?? [];
    v.members = list.map((p) => toMember(p, { ...app.user, user_id: app.user.id }));
  }
});

// The groups, by name: rows of the group table, which everyone reads and owners write. Who is in one is each
// member's groups, so they move with the members too.
export const groups = resource({
  tables: ['archive.group', 'archive.member'],
  load: async () => {
    const v = ws();
    const list = (await api('/api/group?limit=1000', { workspace: v.id })) ?? [];
    v.groups = list.map((g) => ({ id: g.id, name: g.name, description: g.description ?? '' })).sort((a, b) => a.name.localeCompare(b.name));
  }
});

// Where a group is used, for its page: each table with how many rows have it as their access. It moves with any
// row of any table that can have one.
export const usage = resource({
  tables: (c) => !c.table.startsWith('archive.') || !['archive.chat', 'archive.workflow_run', 'archive.revision', 'archive.token', 'archive.user', 'archive.workspace'].includes(c.table),
  load: async (id) => {
    const v = ws();
    v.usage = { ...v.usage, [id]: await groupUses(id) };
  }
});

// Workflows are rows of the workflow table and their runs of workflow_run, read through a run as any rows are.
const runCode = async (code, workspace) => {
  const out = await api('/api/run', { method: 'POST', body: { code }, workspace: workspace ?? app.archive });
  if (out?.error) throw new Error(out.error);
  return out?.result;
};
const workflowList = (workspace) =>
  runCode(
    `const ws = await archive.list("workflow", { limit: 1000 });
const last = await archive.query("SELECT DISTINCT ON (workflow) id, workflow, created_at AS at, trigger, ms, error FROM workflow_run ORDER BY workflow, created_at DESC, id DESC");
return ws.map((w) => ({ ...w, last: last.find((r) => r.workflow === w.id) ?? null }));`,
    workspace
  );

// The workflows, each with its last run. Runs already read for a workflow's page stay until runs reads them.
export const workflows = resource({
  tables: ['archive.workflow', 'archive.workflow_run'],
  load: async () => {
    const v = ws();
    const list = (await workflowList(v.id)) ?? [];
    const had = new Map(v.workflows.map((w) => [w.id, w]));
    v.workflows = list.map((w) => {
      const fresh = toWorkflow(w);
      const old = had.get(fresh.id);
      return old?.loaded ? { ...fresh, runs: old.runs, loaded: true, enabled: old.enabled } : fresh;
    });
  }
});

// A workflow's runs, for its page.
export const runs = resource({
  tables: ['archive.workflow_run', 'archive.workflow'],
  load: async (id) => {
    const wf = workflowById(id);
    if (!wf) return;
    const list = (await runCode(`return await archive.query("SELECT id, created_at AS at, trigger, ms, result, logs, error FROM workflow_run WHERE workflow = $1 ORDER BY created_at DESC, id DESC LIMIT 50", ${JSON.stringify(wf.id)});`)) ?? [];
    const now = workflowById(id) ?? wf;
    const running = now.runs.filter((r) => r.status === 'running');
    now.runs = [...running, ...list.map((r, i) => toRun(r, list.length - i))];
    now.loaded = true;
  }
});

// The tables and views, from the catalog and the DDL: they move with a migration. Which workflows a table
// starts moves with the workflows.
export const schema = resource({
  tables: ['archive.revision', 'archive.workflow'],
  load: async () => {
    const v = ws();
    const [catalog, ddl, flows] = await Promise.all([
      api('/api/types', { workspace: v.id }),
      api('/api/schema', { workspace: v.id }).catch(() => []),
      workflowList(v.id).catch(() => [])
    ]);
    v.catalog = { catalog, ddl, flows: flows ?? [] };
    tables(v);
    await counts.refresh();
  }
});

// Every table's row count and last change, in one read-only query: they move with any row of any table.
export const counts = resource({
  tables: (c) => !c.table.startsWith('archive.'),
  load: async () => {
    const v = ws();
    const sql = v.catalog && countsQuery(v.catalog.catalog);
    if (!sql) return;
    const rows = await api('/api/run', { method: 'POST', body: { code: `return await archive.query(${JSON.stringify(sql)})` }, workspace: v.id });
    if (rows.error) return;
    v.counts = Object.fromEntries((rows.result ?? []).map((r) => [r.name, r]));
    tables(v);
  }
});

// The schema's tables from what was read last, keeping the rows each table's page read.
function tables(v) {
  if (!v.catalog) return;
  const samples = new Map(v.schema.map((t) => [t.id, t.sample]));
  v.schema = toTables(v.catalog.catalog, v.catalog.ddl, v.counts ?? {}, v.catalog.flows).map((t) => ({ ...t, sample: samples.get(t.id) ?? [] }));
  v.archiveTables = toTables(v.catalog.catalog, [], {}, [], true);
}

// The first rows of a table, for its page; they move with any write to it.
export const preview = resource({
  tables: (c) => !c.table.startsWith('archive.'),
  param: (c) => c.table,
  load: async (id) => {
    const rows = (await call(`/api/${encodeURIComponent(id)}?limit=5`).catch(() => [])) ?? [];
    const t = ws().schema.find((x) => x.id === id);
    if (t) t.sample = rows;
  }
});

// A view's rows, as its page draws them: they move with any row of any table, since a view reads others.
export const viewRows = resource({
  tables: (c) => !c.table.startsWith('archive.'),
  param: (c) => c.table,
  load: async (id) => {
    const v = ws();
    const rows = (await call(`/api/${encodeURIComponent(id)}?limit=500`).catch(() => [])) ?? [];
    v.viewRows = { ...v.viewRows, [id]: rows };
  }
});

// tableOf is a table or view of the workspace, or one of Archive's own that a link names, by its id.
export function tableOf(id, v = ws()) {
  return v?.schema.find((x) => x.id === id) ?? v?.archiveTables?.find((x) => x.id === id);
}

// baseOf is the table a view reads, as its DDL names it after its own FROM, past the parentheses Postgres puts
// around joins and past its schema, archive's as the workspace's: not the FROM of a subquery in its select list,
// like a count of another table; a table is its own.
// A view whose FROM is a subquery, as one given its name column reads (objectUpgrade), reads what that reads.
export function baseOf(t) {
  if (t?.kind !== 'view') return t?.id;
  const from = (ddl) => {
    let depth = 0;
    let quoted = false;
    for (let i = 0; i < ddl.length; i++) {
      const ch = ddl[i];
      if (ch === "'") quoted = !quoted;
      if (quoted) continue;
      if (ch === '(') depth++;
      else if (ch === ')') depth--;
      else if (depth === 0 && /^from\b/i.test(ddl.slice(i, i + 5)) && !/\w/.test(ddl[i - 1] ?? '')) {
        const rest = ddl.slice(i + 4);
        const sub = /^\s*\(\s*select\b/i.exec(rest);
        if (!sub) return /^\s+[(\s]*(?:"?\w+"?\.)?"?(\w+)"?/i.exec(rest)?.[1];
        const open = rest.indexOf('(');
        let d = 0;
        for (let j = open; j < rest.length; j++) {
          if (rest[j] === '(') d++;
          else if (rest[j] === ')' && --d === 0) return from(rest.slice(open + 1, j).replace(/^\s*select\b/i, ' '));
        }
        return undefined;
      }
    }
  };
  return from(t.ddl ?? '') ?? t.id;
}

// linkOf is the table a column links to; a view's columns have none of their own, so it is its table's, or that
// of the view it reads.
export function linkOf(t, c, depth = 0) {
  if (c.to || t?.kind !== 'view' || depth > 4) return c.to;
  const base = tableOf(baseOf(t));
  const col = base !== t && base?.columns.find((x) => x.name === c.name);
  return col ? linkOf(base, col, depth + 1) : undefined;
}

// shownOf is the columns of t a screen shows a row by, past its name: not what Archive keeps of every row, and
// not an id that names no table the app knows, which would read as a bare UUID; nor, of Archive's own tables,
// the ids a row has at the service it came from, nor of a view of one.
export function shownOf(t) {
  const own = ['id', 'name', 'access', 'created_at', 'updated_at', 'search'];
  const base = tableOf(baseOf(t));
  const archive = Boolean(base) && !ws()?.schema.includes(base);
  return (t?.columns ?? []).filter(
    (c) =>
      !own.includes(c.name) &&
      (linkOf(t, c) || !['id', 'link', 'file'].includes(c.type)) &&
      !(archive && (c.name === 'source_id' || c.name === 'stream' || c.name.endsWith('_source')))
  );
}

// rowOf is one row of a table or view, as its member may read it.
export async function rowOf(table, id) {
  return call(`/api/${encodeURIComponent(table)}/${encodeURIComponent(id)}`);
}

// titleOf is what a row of t is called: its name, shown as a cell is; for a row of a view with none, the name
// of the row of the table it reads; or Untitled; never its id.
export function titleOf(row, t) {
  const own = show(row?.name).trim();
  if (own || t?.kind !== 'view') return own || 'Untitled';
  const base = baseOf(t);
  return (base !== t.id && tableOf(base) && nameOf(base, row?.id)?.trim()) || 'Untitled';
}

// Row names, for links: by table and id in each workspace's names, a name, or null for a row the member
// cannot see. A link asks for its row's name; what is asked at once is read in one query, as the member may
// read it, and read again when its table moves.
const asked = new Map();
let askTimer = null;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const reading = new Set();

function ask(v, table, id) {
  reading.add(`${v.id}\n${table}\n${id}`);
  const key = `${v.id}\n${table}`;
  if (!asked.has(key)) asked.set(key, { v, table, ids: new Set() });
  asked.get(key).ids.add(id);
  askTimer ??= setTimeout(readNames);
}

async function readNames() {
  askTimer = null;
  const batch = [...asked.values()];
  asked.clear();
  for (const v of new Set(batch.map((b) => b.v))) {
    const mine = batch.filter((b) => b.v === v);
    const tables = v.catalog?.catalog?.tables ?? {};
    const parts = [];
    const params = [];
    for (const b of mine) {
      const t = b.table === 'archive.file' ? { schema: 'archive', name: 'file' } : tables[b.table];
      if (!t) continue;
      params.push([...b.ids]);
      const ident = `"${t.schema}"."${t.name.replaceAll('"', '""')}"`;
      parts.push(`SELECT '${b.table.replaceAll("'", "''")}' AS t, id::text AS id, name::text AS name FROM ${ident} WHERE id = ANY($${params.length}::uuid[])`);
    }
    const code = `return await archive.query(${JSON.stringify(parts.join(' UNION ALL '))}, ...${JSON.stringify(params)});`;
    const rows = parts.length ? await runCode(code, v.id).catch(() => []) : [];
    const got = new Map((rows ?? []).map((r) => [`${r.t}\n${r.id}`, r.name ?? '']));
    for (const b of mine) {
      v.names[b.table] ??= {};
      for (const id of b.ids) {
        v.names[b.table][id] = got.get(`${b.table}\n${id}`) ?? null;
        reading.delete(`${v.id}\n${b.table}\n${id}`);
      }
    }
  }
}

// The names of a table that moved are read again; until then its links show the names they had.
onChange((changes, sweep) => {
  for (const v of app.archives) {
    for (const table of Object.keys(v.names ?? {})) {
      if (sweep || changes.some((c) => c.table === table || c.table === `archive.${table}`)) {
        for (const id of Object.keys(v.names[table])) ask(v, table, id);
      }
    }
  }
});

// nameOf is the name of the row id of table, for a link to it: undefined while it is read, null when the
// member cannot see it.
export function nameOf(table, id, v = ws()) {
  if (!v || !table || typeof id !== 'string' || !UUID.test(id)) return null;
  const got = v.names?.[table]?.[id];
  if (got === undefined && !reading.has(`${v.id}\n${table}\n${id}`)) ask(v, table, id);
  return got;
}

// linkTitle is how a link to a row reads: its name, Untitled, or why there is none to show.
export function linkTitle(table, id) {
  const name = nameOf(table, id);
  return name === undefined ? '…' : name === null ? 'Not found' : name.trim() || 'Untitled';
}

// linkHref is the page of the row a link names: its row's page, or the file's in the library.
export function linkHref(table, id) {
  return table === 'archive.file' ? `/library/${id}` : `/view/${table}/${id}`;
}

// cellText is a cell as text, for search, filters, groups and sorting: a link by the names of its rows.
export function cellText(t, c, row) {
  const v = row?.[c.name];
  const to = linkOf(t, c);
  if (!to || v == null || v === '') return show(v);
  return (Array.isArray(v) ? v : [v]).map((id) => linkTitle(to, id)).join(', ');
}

// What links to a row of view (or table) id: the rows of other tables whose link columns point at the table
// the view reads, which its DDL names. Each is { table, column, rows: [{ id, title }] }.
export async function rowRelations(id, row) {
  const v = ws();
  const t = tableOf(id, v);
  if (!t || !row?.id) return { from: [] };
  const base = baseOf(t);
  const types = [...v.schema, ...v.archiveTables];
  const from = await Promise.all(
    types
      .filter((x) => x.kind !== 'view')
      .flatMap((x) => x.columns.filter((c) => c.to === base).map((c) => ({ x, c })))
      .map(async ({ x, c }) => {
        const rows = (await call(`/api/${encodeURIComponent(x.id)}?${encodeURIComponent(c.name)}=${row.id}&limit=20`).catch(() => [])) ?? [];
        return { table: x.id, column: c.name, rows: rows.map((r) => ({ id: r.id, title: titleOf(r) })) };
      })
  );
  return { from: from.filter((l) => l.rows.length) };
}

export async function loadArchive(v = ws()) {
  v.loading = true;
  await Promise.allSettled([chats.refresh(), library.refresh(), members.refresh(), groups.refresh(), workflows.refresh(), schema.refresh()]);
  v.loading = false;
}

export async function signOut() {
  await apiSignOut();
  tabs?.postMessage('signedOut');
  listen('');
  app.status = 'signedOut';
  app.archives = [];
  app.archive = null;
  go('/');
}

// Settings dialogs

export function openSettings(section = null, sub = null) {
  app.archiveSettings.open = false;
  Object.assign(app.settings, { open: true, section, sub });
}

export function openArchiveSettings(section = null, sub = null) {
  app.settings.open = false;
  Object.assign(app.archiveSettings, { open: true, section, sub });
}

// Onboarding (MOCK: no API; kept per archive in localStorage)

export const onboardingSteps = [
  { id: 'msa', group: 'Legal', title: 'Sign the master services agreement', body: 'The terms for your archive, billing and support.', action: 'Review and sign' },
  { id: 'dpa', group: 'Legal', title: 'Sign the data processing agreement', body: 'How we process personal data on your behalf under GDPR.', action: 'Review and sign' },
  { id: 'verify', group: 'Legal', title: 'Verify your business details', body: 'Confirm your legal name, address and VAT number for invoices.', action: 'Verify' },
  { id: 'invite', group: 'Your team', title: 'Invite your team', body: 'Add the people who should read, edit or own this archive.', action: 'Invite' },
  { id: 'connect', group: 'Your data', title: 'Connect a source', body: 'Bring in your Gmail, Google Drive, Notion and more.', action: 'Connect' },
  { id: 'upload', group: 'Your data', title: 'Upload your first files', body: 'Drop in scans, letters, recordings or spreadsheets.', action: 'Upload' },
  { id: 'workflow', group: 'Your data', title: 'Set up a workflow', body: 'Let an agent file, tag or transcribe new items as they arrive.', action: 'Create' },
  { id: 'region', group: 'Your data', title: 'Choose a data region and retention', body: 'Pick where your archive lives and how long deleted items are kept.', action: 'Choose' }
];

const saveSteps = () => setLocal(`archive.onboarding:${app.archive}`, app.onboarding);

export function stepDone(id) {
  return app.onboarding.includes(id);
}

export function completeStep(id) {
  if (stepDone(id)) return;
  app.onboarding.push(id);
  saveSteps();
}

export function toggleStep(id) {
  const i = app.onboarding.indexOf(id);
  if (i === -1) app.onboarding.push(id);
  else app.onboarding.splice(i, 1);
  saveSteps();
}

// People

export function seedOf(person) {
  const p = typeof person === 'string' ? { name: person } : person;
  if (p.user || p.id) return p.user ?? p.id;
  const m = ws()?.members.find((x) => (p.email && x.email === p.email) || (!p.email && x.name === p.name));
  return m?.user ?? m?.id ?? p.email ?? p.name ?? '';
}

export function isArchiveOwner() {
  return app.role === 'owner';
}

export async function invite({ emails, role, groups: ids = [] }) {
  const w = ws();
  const added = [];
  for (const email of emails) {
    if (w.members.some((m) => m.email === email)) continue;
    try {
      const people = await call('/auth/users', { method: 'POST', body: { email, role } });
      w.members = people.map((p) => toMember(p, { ...app.user, user_id: app.user.id }));
      const m = w.members.find((x) => x.email === email);
      if (m && ids.length) await setGroups(m, ids);
      added.push(m);
    } catch (e) {
      await complain(e, `Couldn’t invite ${email}`);
    }
  }
  if (added.length) completeStep('invite');
  return added.filter(Boolean);
}

export async function removeMember(id) {
  const w = ws();
  const m = w.members.find((x) => x.id === id);
  if (!m) return;
  try {
    const people = await call(`/auth/users/${encodeURIComponent(m.email)}`, { method: 'DELETE' });
    w.members = people.map((p) => toMember(p, { ...app.user, user_id: app.user.id }));
  } catch (e) {
    complain(e, `Couldn’t remove ${m.name}`);
  }
}

export async function setRole(member, role) {
  const before = member.role;
  member.role = role;
  try {
    await call(`/api/member/${member.id}`, { method: 'PATCH', body: { role } });
  } catch (e) {
    member.role = before;
    complain(e, `Couldn’t change ${member.name}’s role`);
  }
}

// Groups. A group is a row of the group table: an owner makes one, even empty, renames, describes and deletes
// it. A member's groups are the ids of theirs, a column of their row; a row's access is the id of the group
// that sees it, or null for everyone.

export function groupById(id, w = ws()) {
  return w?.groups.find((g) => g.id === id);
}

export const groupName = (id) => (id ? (groupById(id)?.name ?? 'Unknown group') : 'Everyone');

export async function setGroups(member, next) {
  const before = member.groups;
  member.groups = [...new Set(next)];
  try {
    await call(`/api/member/${member.id}`, { method: 'PATCH', body: { groups: member.groups } });
    return true;
  } catch (e) {
    member.groups = before;
    complain(e, `Couldn’t change ${member.name}’s groups`);
    return false;
  }
}

// mayLimit says whether you may change who sees a row or file whose access is current: an editor in its group,
// or of one everyone sees, or an owner. accessOptions are what you may give it to: everyone, and the groups you
// are in, or every group for an owner.
export function mayLimit(current) {
  if (app.role === 'owner') return true;
  const me = ws().members.find((m) => m.email === app.user.email);
  return app.role === 'editor' && (!current || Boolean(me?.groups.includes(current)));
}

export function accessOptions(current) {
  const me = ws().members.find((m) => m.email === app.user.email);
  const mine = ws().groups.filter((g) => app.role === 'owner' || me?.groups.includes(g.id) || g.id === current);
  return [{ value: '', label: 'Everyone' }, ...mine.map((g) => ({ value: g.id, label: g.name }))];
}

export const toggleGroup = (member, id) => setGroups(member, member.groups.includes(id) ? member.groups.filter((g) => g !== id) : [...member.groups, id]);

// createGroup makes the group name, described, with the members whose ids are people in it; it is the group, or
// null when it could not be made.
export async function createGroup({ name, description = '', people = [] }) {
  const w = ws();
  let g;
  try {
    g = await call('/api/group', { method: 'POST', body: { name, description } });
  } catch (e) {
    complain(e, `Couldn’t make ${name || 'the group'}`);
    return null;
  }
  const group = { id: g.id, name: g.name, description: g.description ?? '' };
  if (!w.groups.some((x) => x.id === group.id)) w.groups = [...w.groups, group].sort((a, b) => a.name.localeCompare(b.name));
  for (const m of w.members) if (people.includes(m.id)) await setGroups(m, [...m.groups, group.id]);
  return group;
}

export async function updateGroup(group, change) {
  const before = { name: group.name, description: group.description };
  Object.assign(group, change);
  try {
    const g = await call(`/api/group/${group.id}`, { method: 'PATCH', body: change });
    Object.assign(group, { name: g.name, description: g.description ?? '' });
    return true;
  } catch (e) {
    Object.assign(group, before);
    complain(e, `Couldn’t change ${before.name}`);
    return false;
  }
}

export async function deleteGroup(group) {
  try {
    await call(`/api/group/${group.id}`, { method: 'DELETE' });
  } catch (e) {
    complain(e, `Couldn’t delete ${group.name}`);
    return false;
  }
  const w = ws();
  w.groups = w.groups.filter((g) => g.id !== group.id);
  for (const m of w.members) m.groups = m.groups.filter((g) => g !== group.id);
  return true;
}

// groupUses is where the group id is a row's access: each table, as the catalog names it, with how many rows
// you see there, and the connections narrowed to it. While it has any, it cannot be deleted.
export async function groupUses(id) {
  const rows = (await runCode(`return await archive.query("select * from archive.group_uses($1)", ${JSON.stringify(id)});`)) ?? [];
  return rows.map((r) => ({ table: r.table, rows: Number(r.rows) }));
}

// setAccess gives the row id of table to the group access, or to everyone with null.
export async function setAccess(table, row, access) {
  const before = row.access;
  row.access = access;
  try {
    await call(`/api/${encodeURIComponent(table)}/${row.id}`, { method: 'PATCH', body: { access } });
    return true;
  } catch (e) {
    row.access = before;
    complain(e, 'Couldn’t change who sees it');
    return false;
  }
}

// Integrations: the archive's connections, one to each integration the API has, which an owner connects for
// everyone (src/connect). They are read with your own connectors, from /connect.

export function archiveIntegrations() {
  return ws().integrations ?? [];
}

export function connectIntegration(id) {
  if (isArchiveOwner()) connect(id);
}

async function onIntegration(id, what, path, init) {
  const c = archiveIntegrations().find((x) => x.id === id);
  if (!c?.connection || !isArchiveOwner()) return;
  try {
    await call(`/connect/${c.connection.id}${path}`, init);
  } catch (e) {
    complain(e, `Couldn’t ${what} ${c.name}`);
  }
  return connections.refresh();
}

export const disconnectIntegration = (id) => onIntegration(id, 'disconnect', '', { method: 'DELETE' });
export const syncIntegration = (id) => onIntegration(id, 'sync', '/sync', { method: 'POST' });

// API keys. The archive key is a real API token of yours for this archive; its secret is shown once, when it is
// made. "Your API key", one key across every archive, is MOCK: tokens belong to one workspace.

const KEY_NAME = 'Archive API key';

export function archiveKey() {
  return app.archiveKeys[app.archive] ?? null;
}

// Your tokens for the archive, read again when one is made or revoked, here or anywhere else.
export const tokens = resource({
  tables: ['archive.token'],
  load: async () => {
    const id = app.archive;
    const list = (await call('/auth/tokens')) ?? [];
    // The apps you let in over MCP (src/mcp.go), each a grant you can revoke.
    app.archiveApps[id] = list.filter((x) => x.app).sort((a, b) => b.created_at.localeCompare(a.created_at));
    const t = list.filter((x) => x.name === KEY_NAME && !x.app).sort((a, b) => b.created_at.localeCompare(a.created_at))[0];
    if (app.archiveKeys[id]?.secret && app.archiveKeys[id].id === t?.id) return;
    // Archive keeps the key sealed, so it can be read again; a key made before that has no secret to read.
    const secret = t ? ((await call(`/auth/tokens/${t.id}/secret`).catch(() => null))?.secret ?? '') : '';
    app.archiveKeys[id] = t ? { id: t.id, secret, date: `Created ${new Date(t.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}` } : null;
  }
});

export function archiveApps() {
  return app.archiveApps[app.archive] ?? [];
}

// Revoking an app ends its access and refresh tokens at once.
export async function revokeApp(id) {
  const before = app.archiveApps[app.archive];
  app.archiveApps[app.archive] = archiveApps().filter((x) => x.id !== id);
  try {
    await call(`/auth/tokens/${id}`, { method: 'DELETE' });
  } catch (e) {
    app.archiveApps[app.archive] = before;
    complain(e, 'Couldn’t disconnect it');
  }
}

export async function rotateKey(scope) {
  if (scope !== 'archive') {
    app.apiKey = mockKey('ak_live_', 'Rotated just now');
    return;
  }
  if (!isArchiveOwner()) return;
  const id = app.archive;
  try {
    const old = (await call('/auth/tokens')).filter((x) => x.name === KEY_NAME);
    const t = await call('/auth/tokens', { method: 'POST', body: { name: KEY_NAME } });
    for (const o of old) await call(`/auth/tokens/${o.id}`, { method: 'DELETE' }).catch(() => {});
    app.archiveKeys[id] = { id: t.id, secret: t.token, date: old.length ? 'Rotated just now' : 'Created just now' };
  } catch (e) {
    complain(e, 'Couldn’t make a new key');
  }
}

// Layout

export function isNarrow() {
  return typeof matchMedia !== 'undefined' && matchMedia('(max-width: 56rem)').matches;
}

export function closeDrawer() {
  if (!app.drawer) return;
  const inside = document.activeElement?.closest?.('.sidebar');
  app.drawer = false;
  if (inside) requestAnimationFrame(() => document.querySelector('main button[aria-label="Open sidebar"]')?.focus());
}

export function toggleSidebar() {
  if (isNarrow()) {
    if (app.drawer) closeDrawer();
    else app.drawer = true;
    return;
  }
  app.collapsed = !app.collapsed;
  localStorage.setItem('archive.sidebar', app.collapsed ? 'collapsed' : 'open');
}

function removeAt(list, predicate, after) {
  const i = list.findIndex(predicate);
  if (i < 0) return null;
  const [item] = list.splice(i, 1);
  after?.(item);
  return item;
}

// Archives

export async function switchArchive(id) {
  if (id === app.archive || !app.archives.some((v) => v.id === id)) return;
  setWorkspace(id);
  app.archive = id;
  app.onboarding = local(`archive.onboarding:${id}`, []);
  const head = route.parts[0];
  if (['chat', 'workflows', 'database'].includes(head) && route.parts[1]) go(`/${head === 'chat' ? '' : head}`);
  // Listening to the archive reads who you are in it (me) and what is on screen.
  listen(id);
  loadArchive(ws());
}

export async function createArchive(name, look) {
  if (!app.canAdd) {
    await notify({ title: 'Ask an admin for a new archive', body: 'Only the people who run this Archive can make archives. They add one with archive workspace add and name you as its owner.' });
    return;
  }
  try {
    const w = await api('/auth/workspaces', { method: 'POST', body: { name, owner: app.user.email, ...lookBody(look) }, workspace: app.archives[0].id });
    if (!app.archives.some((v) => v.id === w.id)) app.archives.push(shell(w));
    await switchArchive(w.id);
    go('/');
  } catch (e) {
    complain(e, 'Couldn’t make the archive');
  }
}

// Saves the archive's name and look in one PATCH, the one a rename makes, which tells every tab it moved.
export async function saveArchive({ name, look }) {
  const w = ws();
  const before = { name: w.name, look: w.look, kind: w.kind };
  Object.assign(w, { name: name || w.name, look: { ...look }, kind: kindOf(look.type) });
  try {
    const saved = await call('/auth/workspace', { method: 'PATCH', body: { name: w.name, ...lookBody(look) } });
    Object.assign(w, { name: saved.name, look: lookOf(saved), kind: kindOf(saved.type) });
  } catch (e) {
    Object.assign(w, before);
    complain(e, 'Couldn’t save the archive');
  }
}

export async function deleteArchive(id) {
  if (app.archives.length < 2) return;
  try {
    await api(`/auth/workspaces/${id}`, { method: 'DELETE', workspace: app.archives.find((v) => v.id !== id).id });
  } catch (e) {
    return complain(e, 'Couldn’t delete the archive');
  }
  app.archives.splice(app.archives.findIndex((w) => w.id === id), 1);
  if (app.archive === id) {
    app.archive = null;
    await switchArchive(app.archives[0].id);
  }
  go('/');
}

// Chats. Sending and streaming live in chat.svelte.js.

export function newChat(title, { attachments = [], text } = {}) {
  const id = nid('new-');
  const w = ws();
  w.chats.unshift({ id, title: title || 'New chat', updated: 'Just now', draft: true });
  w.conversations[id] = [];
  if (text !== undefined) app.pending = { id, text, attachments };
  return id;
}

export function chatById(id) {
  return ws()?.chats.find((c) => c.id === id);
}

export async function renameChat(id, title) {
  const chat = chatById(id);
  if (!chat || !title) return;
  const before = chat.title;
  chat.title = title;
  if (chat.draft) return;
  try {
    chat.title = (await call(`/api/chats/${id}`, { method: 'PATCH', body: { title } })).title || title;
  } catch (e) {
    chat.title = before;
    complain(e, 'Couldn’t rename the chat');
  }
}

// Pins are MOCK: chats have no pinned flag in the API, so pins stay on this device.
export function togglePin(id) {
  const chat = chatById(id);
  if (!chat) return;
  chat.pinned = !chat.pinned;
  setLocal(`archive.pins:${app.archive}`, ws().chats.filter((c) => c.pinned && !c.draft).map((c) => c.id));
}

export function pinnedFirst(chats) {
  return [...chats.filter((c) => c.pinned), ...chats.filter((c) => !c.pinned)];
}

export async function removeChat(id) {
  const w = ws();
  const chat = chatById(id);
  if (!chat) return;
  const open = route.path === `/chat/${id}`;
  if (!chat.draft) {
    try {
      await call(`/api/chats/${id}`, { method: 'DELETE' });
    } catch (e) {
      return complain(e, 'Couldn’t delete the chat');
    }
  }
  removeAt(w.chats, (c) => c.id === id, () => open && go('/'));
}

// Library

export async function uploadFiles(list, folder) {
  const w = ws();
  const access = w.groups.find((g) => g.name === folder)?.id;
  const added = [];
  for (const file of list) {
    try {
      const f = filed(toFile(await upload(file, access), w.name), w);
      w.library.unshift(f);
      if (!w.folders.includes(f.folder)) w.folders.push(f.folder);
      added.push(f);
    } catch (e) {
      await complain(e, `Couldn’t upload ${file.name}`);
    }
  }
  if (added.length) completeStep('upload');
  return added;
}

// A file's folder is the name of the group it is limited to, or Everyone.
function filed(f, w) {
  f.folder = f.access ? (groupById(f.access, w)?.name ?? 'Unknown group') : 'Everyone';
  return f;
}

// setFileAccess limits file to the group access, or opens it to everyone with null.
export async function setFileAccess(file, access) {
  const w = ws();
  try {
    const f = filed(toFile(await call(`/api/files/${file.id}`, { method: 'PATCH', body: { access } }), w.name), w);
    Object.assign(file, { access: f.access, folder: f.folder });
    if (!w.folders.includes(f.folder)) w.folders.push(f.folder);
  } catch (e) {
    complain(e, `Couldn’t change who sees ${file.name}`);
  }
}

// Folders are the group a file is limited to ("Everyone" when it isn't). A new, empty one is MOCK
// until a file is filed in it.
export function addFolder(name) {
  const w = ws();
  if (!name || w.folders.includes(name)) return;
  w.folders.push(name);
}

export async function renameFile(id, name) {
  const f = ws().library.find((x) => x.id === id);
  if (!f || !name) return;
  const before = f.name;
  f.name = name;
  try {
    await call(`/api/files/${id}`, { method: 'PATCH', body: { name } });
  } catch (e) {
    f.name = before;
    complain(e, 'Couldn’t rename the file');
  }
}

export async function removeFile() {
  await notify({ title: 'Files can’t be deleted yet', body: 'Archive keeps every file for now. Deleting from the library comes with an API for it.' });
}

export async function downloadFile(file) {
  try {
    const href = await blobURL(`/api/files/${file.id}/content`, app.archive);
    Object.assign(document.createElement('a'), { href, download: file.name }).click();
  } catch (e) {
    complain(e, `Couldn’t download ${file.name}`);
  }
}

// Sharing (MOCK: there is no API for sharing one item with someone outside the archive; shares stay in this tab)

export const permissions = [
  { value: 'view', label: 'Viewer' },
  { value: 'edit', label: 'Editor' }
];

export const shareKinds = { file: 'File', folder: 'Folder', workflow: 'Workflow', table: 'Table' };

export function shareItem(kind, target, w = ws()) {
  if (kind === 'file') {
    const f = w.library.find((x) => x.id === target);
    return f && { name: f.name, path: `/library/${f.id}` };
  }
  if (kind === 'workflow') {
    const f = w.workflows.find((x) => x.id === target);
    return f && { name: f.name, path: `/workflows/${f.id}` };
  }
  if (kind === 'table') {
    const t = w.schema.find((x) => x.id === target);
    return t && { name: t.name, path: `/database/${t.id}` };
  }
  return w.folders.includes(target) ? { name: target, path: '/library' } : null;
}

export function isExternal(email) {
  return !ws().members.some((m) => m.email === email);
}

export function shareFor(kind, target) {
  return ws().shares.find((s) => s.kind === kind && s.target === target);
}

function ensureShare(kind, target) {
  const w = ws();
  if (!shareFor(kind, target)) w.shares.unshift({ id: nid('sh'), kind, target, by: app.profile.name, date: 'Just now', people: [] });
  return shareFor(kind, target);
}

function prune(share) {
  if (share.people.length) return;
  const w = ws();
  const i = w.shares.indexOf(share);
  if (i >= 0) w.shares.splice(i, 1);
}

export function openShare(kind, target) {
  Object.assign(app.share, { open: true, kind, target });
}

export function contacts(w = ws()) {
  const out = new Map();
  for (const m of w.members) {
    if (m.status === 'active' && m.email !== app.user.email) out.set(m.email, { id: m.id, user: m.user, name: m.name, email: m.email, tone: m.tone });
  }
  for (const s of w.shares) {
    for (const p of s.people) if (!out.has(p.email)) out.set(p.email, { name: p.name, email: p.email, tone: p.tone, guest: true });
  }
  return [...out.values()];
}

export function addPeople(kind, target, emails, permission) {
  const share = ensureShare(kind, target);
  const known = contacts();
  const added = [];
  for (const email of emails) {
    if (share.people.some((p) => p.email === email)) continue;
    const person = known.find((p) => p.email === email);
    const name = person?.name ?? email.split('@')[0].replace(/[._-]+/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
    share.people.push({ name, email, permission, tone: person?.tone ?? (email.length % 6) + 1 });
    added.push(name);
  }
  if (!added.length) prune(share);
}

export function setPermission(share, email, permission) {
  const p = share.people.find((x) => x.email === email);
  if (p) p.permission = permission;
}

export function unsharePerson(share, email) {
  const i = share.people.findIndex((x) => x.email === email);
  if (i < 0) return;
  share.people.splice(i, 1);
  prune(share);
}

export function stopSharing(id) {
  removeAt(ws().shares, (s) => s.id === id);
}

export function removeGuest(email) {
  const w = ws();
  for (const share of [...w.shares]) {
    share.people = share.people.filter((p) => p.email !== email);
    prune(share);
  }
}

// Workflows. A workflow is a script saved in the archive's workflow table; runs are synchronous, so there is no pausing or
// stopping one, and the assistant writes new ones.

export function workflowById(id) {
  return ws()?.workflows.find((w) => w.id === id);
}

export function runWorkflow(id) {
  const wf = workflowById(id);
  if (!wf || wf.runs[0]?.status === 'running') return;
  const number = (wf.runs[0]?.number ?? 0) + 1;
  const placeholder = { id: nid('r'), number, status: 'running', started: 'Just now', elapsed: '', seconds: 0, steps: [{ id: 's0', label: `Running ${wf.file}`, detail: '', status: 'running', time: '' }], log: [] };
  wf.runs.unshift(placeholder);
  const index = () => wf.runs.findIndex((x) => x.id === placeholder.id);
  const at = new Date();
  runCode(`return await archive.run(${JSON.stringify(wf.file)});`)
    .then((r) => {
      const done = toRun({ id: placeholder.id, at: at.toISOString(), ms: Date.now() - at, trigger: 'run', ...r }, number);
      if (index() >= 0) wf.runs[index()] = { ...done, id: placeholder.id };
    })
    .catch((e) => {
      if (index() >= 0) wf.runs[index()] = { ...placeholder, status: 'failed', steps: [{ ...placeholder.steps[0], status: 'failed', detail: e.message }] };
    });
  return placeholder.id;
}

export function createWorkflow({ name, trigger, steps }) {
  completeStep('workflow');
  const text = `Write and save a workflow called “${name}”.\nTrigger: ${trigger.label}.\nSteps:\n${steps.map((s) => `- ${s}`).join('\n')}`;
  return `/chat/${newChat(name || 'New workflow', { text })}`;
}

// renameView renames view id to name, as an owner changes the schema: a dry run, then the run it confirms. A
// view's name is its Postgres name, so name becomes one, like "Client notes" client_notes; its page follows it.
export async function renameView(id, name) {
  const to = name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
  if (!to || to === id || !isArchiveOwner()) return;
  if (!/^[a-z]/.test(to)) return complain(new Error('A view’s name starts with a letter.'), 'Couldn’t rename the view');
  const sql = `ALTER VIEW public."${id}" RENAME TO "${to}"`;
  try {
    const dry = await call('/api/schema/migrate', { method: 'POST', body: { sql } });
    await call('/api/schema/migrate', { method: 'POST', body: { sql, confirm: dry.confirm } });
  } catch (e) {
    return complain(e, 'Couldn’t rename the view');
  }
  await schema.refresh();
  if (route.path.startsWith(`/view/${id}`)) replace(route.path.replace(`/view/${id}`, `/view/${to}`));
}

export async function renameWorkflow() {
  await notify({ title: 'Ask the assistant', body: 'Ask the assistant to rename the workflow; it keeps its runs, webhook and schedule.' });
}

export async function removeWorkflow(id) {
  const wf = workflowById(id);
  if (!wf) return;
  try {
    await call(`/api/workflow/${encodeURIComponent(wf.id)}`, { method: 'DELETE' });
    removeAt(ws().workflows, (w) => w.id === id, () => go('/workflows'));
  } catch (e) {
    complain(e, 'Couldn’t delete the workflow');
  }
}

// Connectors: only those the API has, with your connections to them, read from /connect.

// Your connected accounts, onto the connectors the API has. Connecting is the provider's consent page, which
// sends the browser back to the connector here.
export const connections = resource({
  tables: ['archive.connection'],
  load: async () => {
    const v = ws();
    const out = await api('/connect', { workspace: v.id });
    // Your own connectors are the personal ones; the archive's integrations are listed in its settings.
    const ids = new Set((out?.connectors ?? []).filter((k) => !k.integration).map((k) => k.id));
    v.connectors = v.connectors.filter((c) => ids.has(c.id));
    v.integrations = (out?.connectors ?? []).filter((k) => k.integration).map((k) => toIntegration(k, out.connections.find((x) => x.provider === k.id && x.integration)));
    for (const k of (out?.connectors ?? []).filter((k) => !k.integration)) {
      let c = v.connectors.find((x) => x.id === k.id);
      if (!c) {
        c = { id: k.id, name: k.name, by: k.name, description: `Use your ${k.name} account`, category: 'Other', scopes: [] };
        v.connectors.push(c);
        c = v.connectors.at(-1);
      }
      const mine = out.connections.filter((x) => x.provider === k.id && !x.integration);
      // Synced is when every account last caught up: the earliest, and never while one has not yet.
      const synced = mine.every((x) => x.synced_at) ? mine.map((x) => x.synced_at).sort()[0] : '';
      Object.assign(c, {
        ready: k.ready,
        connections: mine,
        connected: mine.length > 0,
        account: mine.map((x) => x.account).join(', '),
        error: mine.find((x) => x.error)?.error ?? '',
        lastSync: synced ? relative(synced) : mine.some((x) => x.sync_at) ? 'Syncing…' : 'Not yet',
        items: mine.reduce((n, x) => n + (x.items ?? 0), 0)
      });
    }
    if (v.connectors.some((c) => c.connected)) completeStep('connect');
  }
});

// lastSyncOf is when a connection last caught up, as the app says it.
function lastSyncOf(x) {
  return x.synced_at ? relative(x.synced_at) : x.sync_at ? 'Syncing…' : 'Not yet';
}

// toIntegration is integration connector k as its settings show it, with the archive's connection x, if any.
function toIntegration(k, x) {
  const shown = catalog.find((c) => c.id === k.id) ?? { by: k.name, description: `Use the archive’s ${k.name} account`, category: 'Other', scopes: [] };
  return {
    ...shown,
    id: k.id,
    name: k.name,
    ready: k.ready,
    connected: Boolean(x),
    connection: x ?? null,
    account: x?.account ?? '',
    connectedBy: x?.connected_by ?? '',
    date: x ? relative(x.created_at) : '',
    lastSync: x ? lastSyncOf(x) : '',
    items: x?.items ?? 0,
    error: x?.error ?? '',
    access: x?.access ?? ''
  };
}

// The types of archive one of your own connections is in, which you pick before connecting it: only archives of
// its type read what it syncs and act with it.
export const connectionTypes = [
  { value: 'personal', label: 'Personal' },
  { value: 'family', label: 'Family' },
  { value: 'business', label: 'Business' }
];

// connect sends the browser to connect an account of your own at connector id, into your archives of type.
export function connect(id, type) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c?.ready || !connectionTypes.some((t) => t.value === type)) return;
  location.assign(`${API}/connect/${encodeURIComponent(id)}?${new URLSearchParams({ workspace: app.archive, type })}`);
}

// syncNow asks for a sync of each of your accounts at connector id now, which the API queues.
export async function syncNow(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  try {
    for (const x of c.connections ?? []) await call(`/connect/${x.id}/sync`, { method: 'POST' });
  } catch (e) {
    complain(e, `Couldn’t sync ${c.name}`);
  }
  return connections.refresh();
}

// setConnectionType puts your one account connection in your archives of type, personal, family or business, and
// out of the others, with what it synced.
export async function setConnectionType(connection, type) {
  const before = connection.type;
  connection.type = type;
  try {
    await call(`/connect/${connection.id}/type`, { method: 'PUT', body: { type } });
  } catch (e) {
    connection.type = before;
    complain(e, `Couldn’t move ${connection.account}`);
  }
  return connections.refresh();
}

// disconnectAccount disconnects your one account connection, in every archive you are in, with what it synced.
export async function disconnectAccount(connection) {
  try {
    await call(`/connect/${connection.id}`, { method: 'DELETE' });
  } catch (e) {
    complain(e, `Couldn’t disconnect ${connection.account}`);
  }
  return connections.refresh();
}

export async function disconnect(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  try {
    for (const x of c.connections ?? []) await call(`/connect/${x.id}`, { method: 'DELETE' });
  } catch (e) {
    complain(e, `Couldn’t disconnect ${c.name}`);
  }
  return connections.refresh();
}

export { initials };
