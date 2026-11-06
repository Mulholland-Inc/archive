
import { API, api, firstBoot, forgetBlobs, upload, setWorkspace, workspace, local, setLocal, saveDraft, signOut as apiSignOut, Unauthorized, passkeys, passkeyAsked } from './api.js';
import { resource, listen, onChange, onRefused } from './stream.svelte.js';
import { toFile, toMember, toWorkflow, toRun, relative, show } from './live.js';
import { toTables, countsQuery } from './schema.js';
import { nameBrands } from './brands.svelte.js';
import { lookOf } from './looks.js';
import { notify } from './dialogs.svelte.js';
import { go, replace, route } from './router.svelte.js';

let counter = 0;
const nid = (prefix) => `${prefix}${Date.now().toString(36)}${(counter++).toString(36)}`;

export const pinned = $state({ title: null });

function noSignup() {
  return { terms: {}, models: {}, agreement: [], business: [], billing: [] };
}

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
  canvas: null,
  help: false,
  pending: null,
  muted: false,
  developer: typeof localStorage !== 'undefined' && localStorage.getItem('archive.developer') === 'on',
  settings: { open: false, section: null, sub: null },
  apiKey: null,
  archiveKeys: {},
  apps: [],
  signup: noSignup(),
  ownConnections: []
});

const kindOf = (type) => ({ family: 'Family', business: 'Business', personal: 'Just you' })[type] ?? 'Archive';

const lookBody = (look) => ({ type: look.type, icon: look.icon, color: look.tint });

function shell(w) {
  const look = lookOf(w);
  return {
    id: w.id,
    name: w.name,
    slug: w.slug,
    kind: kindOf(look.type),
    tone: (w.id.charCodeAt(w.id.length - 1) % 6) + 1,
    look,
    agents: [],
    conversations: {},
    library: [],
    sources: [],
    filed: false,
    unread: w.unread ?? false,
    workflows: [],
    members: [],
    groups: [],
    usage: {},
    schema: [],
    archiveTables: [],
    names: {},
    connectors: [],
    flows: [],
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

export async function boot(booting = firstBoot) {
  const query = new URLSearchParams(location.search);
  const named = query.get('workspace');
  if (query.get('error')) notify({ title: 'That didn’t connect', body: query.get('error') });
  if (named) {
    setWorkspace(named);
    history.replaceState(null, '', location.pathname);
  }
  let data;
  try {
    data = await booting;
  } catch (e) {
    if (!(e instanceof Unauthorized) && workspace() && e.status >= 403) {
      setWorkspace('');
      return boot(api('/api/boot'));
    }
    if (!(e instanceof Unauthorized)) throw e;
    app.status = 'signedOut';
    return;
  }
  const { me } = data;
  setMe(me);
  setWorkspace(me.workspace);
  app.archives = me.workspaces.map(shell);
  app.archive = me.workspace;
  app.askPasskey = passkeys && !passkeyAsked(me.actor.user_id);
  app.status = 'ready';
  prepare(ws(), Promise.resolve(data));
  listen(app.archive, data.live);
  account.seen();
  loadArchive(ws());
  resumeLink();
}

async function loadMe() {
  if (app.status !== 'ready') return;
  let me;
  let here = true;
  try {
    ({ me } = await api('/api/boot?only=me', { workspace: app.archive }));
  } catch (e) {
    if (e instanceof Unauthorized) return leave();
    here = false;
    setWorkspace('');
    me = (await api('/api/boot?only=me').catch((e) => (e instanceof Unauthorized ? leave() : null)))?.me;
    if (!me) return setWorkspace(app.archive);
  }
  if (here) setMe(me);
  const had = new Map(app.archives.map((v) => [v.id, v]));
  app.archives = me.workspaces.map((w) => {
    const v = had.get(w.id) ?? shell(w);
    Object.assign(v, { name: w.name, slug: w.slug, look: lookOf(w), kind: kindOf(w.type), unread: (w.unread ?? false) && w.id !== app.archive });
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

onRefused(() => app.status === 'ready' && account.refresh());

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
  app.canAdd = !!me.admin;
}

async function files(id, where) {
  const all = [];
  for (let after = ''; ; ) {
    const next = await api(`/api/file?limit=500&${where}${after ? `&after=${after}` : ''}`, { workspace: id });
    all.push(...next);
    if (next.length < 500) break;
    after = next.at(-1).id;
  }
  return all.reverse();
}


async function booted(v, key, load) {
  const data = await v.boot;
  if (!data || !(key in data)) return load();
  const out = data[key];
  delete data[key];
  return out;
}

export const agents = resource({
  tables: ['archive.agent'],
  load: async () => {
    const v = ws();
    const list = (await booted(v, 'agents', () => api('/api/agent_list?limit=1000', { workspace: v.id }))) ?? [];
    const had = new Map(v.agents.map((a) => [a.id, a]));
    const ids = new Set(list.map((a) => a.id));
    const seen = local(`archive.seen:${v.id}`, {});
    for (const a of list) {
      a.at ??= '';
      a.seen = seen[a.id] ??= a.at;
    }
    setLocal(`archive.seen:${v.id}`, seen);
    v.agents = list.map((a) => (had.has(a.id) ? Object.assign(had.get(a.id), a) : a));
    for (const id of Object.keys(v.conversations)) if (!ids.has(id)) delete v.conversations[id];
  }
});

const SOURCES = `return await archive.query("SELECT c.id, c.provider, c.account FROM archive.connection c WHERE EXISTS (SELECT FROM archive.file f WHERE f.connection = c.id) ORDER BY c.provider, c.account, c.id");`;

export const library = resource({
  tables: ['archive.file', 'archive.group'],
  load: async () => {
    const v = ws();
    try {
      if (!v.catalog) await schema.refresh();
      const [list, sources] = await Promise.all([files(v.id, 'parent=&connection='), runCode(SOURCES, v.id), groups.refresh()]);
      v.library = list.map((f) => filed(toFile(f, v.name), v));
      const named = new Map(v.catalog.catalog.connectors.map((k) => [k.id, k.name]));
      v.sources = sources.map((s) => ({ ...s, name: named.get(s.provider) ?? s.provider }));
    } finally {
      v.filed = true;
    }
  }
});

export async function folderFiles({ parent, connection }) {
  const v = ws();
  const list = await files(v.id, parent ? `parent=${parent}` : `parent=&connection=${connection}`);
  return list.map((f) => filed(toFile(f, v.name), v));
}

export async function fileById(id) {
  const v = ws();
  return filed(toFile(await api(`/api/file/${id}`, { workspace: v.id }), v.name), v);
}

export async function findFiles(q) {
  const v = ws();
  const hits = await api(`/api/search?type=file&limit=100&q=${encodeURIComponent(q)}`, { workspace: v.id });
  return (hits ?? []).map((h) => ({ ...filed(toFile(h.data, v.name), v), snippet: h.snippet }));
}

export const canvas = resource({
  tables: ['archive.file'],
  byId: true,
  load: async (id) => {
    if (app.canvas?.id !== id) return;
    const v = ws();
    let f;
    try {
      f = await fileById(id);
    } catch (e) {
      if (app.canvas?.id !== id) return;
      const gone = [403, 404].includes(e.status);
      if (gone && app.canvas.file) closeFile();
      else app.canvas.error = gone ? 'Not found' : e.message;
      return;
    }
    let { model, error, file } = app.canvas;
    if (f.office && f.held && f.hash !== file?.hash) {
      try {
        model = await api(`/api/file/${id}/model`, { workspace: v.id });
        error = '';
      } catch (e) {
        error = e.message;
      }
    }
    if (app.canvas?.id === id) Object.assign(app.canvas, { file: f, model, error });
  }
});

export function openFile(id, name = '') {
  if (app.canvas?.id === id) return;
  app.canvas = { id, name, file: null, model: null, error: '' };
  canvas.refresh(id);
}

export function openDocument(id, name, model, { footer, fills, download } = {}) {
  app.canvas = { id, name, doc: true, footer, fills, download, file: null, model, error: '' };
}

export function closeFile() {
  app.canvas = null;
}

export async function editFile(id, ops) {
  try {
    await call(`/api/file/${id}`, { method: 'PATCH', body: { edits: ops } });
  } catch (e) {
    throw new Error(e.message.replace(/^archive\.edit: edit \d+ \(\w+\): /, '').replace(/: (bad request|forbidden)$/, ''));
  }
  await canvas.refresh(id);
}

export async function copyFile(id) {
  try {
    const copy = await runCode(`return await archive.copy(${JSON.stringify(id)});`);
    openFile(copy.id);
  } catch (e) {
    complain(e, 'Couldn’t copy the file');
  }
}

onChange((changes) => {
  for (const c of changes) if (c.table === 'archive.file') forgetBlobs(c.id ? `/api/file/${c.id}/` : '/api/file/');
});

const memberOf = (p) => toMember(p, { ...app.user, user_id: app.user.id });

export const members = resource({
  tables: ['archive.member', 'archive.user'],
  load: async () => {
    const v = ws();
    const list = (await booted(v, 'members', () => api('/api/member?limit=1000', { workspace: v.id }))) ?? [];
    v.members = list.map(memberOf).sort((a, b) => a.email.localeCompare(b.email));
  }
});

export const groups = resource({
  tables: ['archive.group', 'archive.member'],
  load: async () => {
    const v = ws();
    const list = (await api('/api/group?limit=1000', { workspace: v.id })) ?? [];
    v.groups = list.map((g) => ({ id: g.id, name: g.name, description: g.description ?? '' })).sort((a, b) => a.name.localeCompare(b.name));
  }
});

export const usage = resource({
  tables: (c) => !c.table.startsWith('archive.') || !['archive.agent', 'archive.turn', 'archive.workflow_run', 'archive.revision', 'archive.token', 'archive.user', 'archive.workspace'].includes(c.table),
  load: async (id) => {
    const v = ws();
    v.usage = { ...v.usage, [id]: await groupUses(id) };
  }
});

const runCode = async (code, workspace) => {
  const out = await api('/api/run', { method: 'POST', body: { code }, workspace: workspace ?? app.archive });
  if (out?.error) throw new Error(out.error);
  return out?.result;
};
export const workflows = resource({
  tables: ['archive.workflow', 'archive.workflow_run'],
  load: async () => {
    const v = ws();
    const list = (await booted(v, 'workflows', () => api('/api/workflow_list?limit=1000', { workspace: v.id }))) ?? [];
    v.flows = list;
    if (v.catalog) {
      v.catalog.flows = list;
      tables(v);
    }
    const had = new Map(v.workflows.map((w) => [w.id, w]));
    v.workflows = list.map((w) => {
      const fresh = toWorkflow(w);
      const old = had.get(fresh.id);
      return old?.loaded ? { ...fresh, runs: old.runs, loaded: true, enabled: old.enabled } : fresh;
    });
  }
});

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

export const schema = resource({
  tables: ['archive.revision'],
  load: async () => {
    const v = ws();
    const [catalog] = await Promise.all([booted(v, 'types', async () => (await api('/api/boot?only=types', { workspace: v.id })).types), workflows.refresh()]);
    v.catalog = { catalog, ddl: catalog.schema, flows: v.flows };
    tables(v);
    counts.reset();
  }
});

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

function tables(v) {
  if (!v.catalog) return;
  const samples = new Map(v.schema.map((t) => [t.id, t.sample]));
  v.schema = toTables(v.catalog.catalog, v.catalog.ddl, v.counts ?? {}, v.catalog.flows).map((t) => ({ ...t, sample: samples.get(t.id) ?? [] }));
  v.archiveTables = toTables(v.catalog.catalog, {}, {}, [], true);
}

export const preview = resource({
  tables: (c) => !c.table.startsWith('archive.'),
  param: (c) => c.table,
  load: async (id) => {
    const rows = (await call(`/api/${encodeURIComponent(id)}?limit=5`).catch(() => [])) ?? [];
    const t = ws().schema.find((x) => x.id === id);
    if (t) t.sample = rows;
  }
});

export const VIEW_PAGE = 500;
const NO_FILTER = ['card', 'us_ssn', 'national', 'passport', 'geo', 'tsvector'];
const views = $state({});
const blank = { query: '', sort: '', page: 1, filters: {} };

export const viewState = (id) => views[id] ?? blank;

export function filterKind(c) {
  if (NO_FILTER.includes(c.sql) || c.type === 'file') return null;
  if (c.type === 'long') return 'contains';
  if (c.list) return 'values';
  if (c.type === 'money' || c.type === 'number') return 'range';
  if (c.type === 'date' && ['date', 'timestamptz'].includes(c.sql)) return 'dates';
  return 'values';
}

const quoted = (name) => `"${name.replaceAll('"', '""')}"`;

function relation(name) {
  const [schema, table] = name.includes('.') ? name.split('.') : [ws().catalog.catalog.tables[name]?.schema ?? 'public', name];
  return `${quoted(schema)}.${quoted(table)}`;
}

const sourceOf = (t) => (t.kind === 'view' ? `(SELECT * FROM ${relation(t.id)} OFFSET 0)` : relation(t.id));
const like = (s) => `%${s.replace(/[\\%_]/g, '\\$&')}%`;

function textOf(t, c, of) {
  const to = linkOf(t, c);
  return to && to !== 'archive.file' && !c.list ? `(SELECT x.name FROM ${relation(to)} x WHERE x.id = ${of})` : `${of}::text`;
}

function whereOf(t, state, skip) {
  const params = [];
  const bind = (v) => `$${params.push(v)}`;
  const all = [];
  for (const [name, f] of Object.entries(state.filters)) {
    const c = t.columns.find((x) => x.name === name);
    if (!c || name === skip) continue;
    const col = `t.${quoted(name)}`;
    if (f.values?.length) {
      const some = f.values.filter((v) => v !== null);
      const set = `(SELECT jsonb_array_elements(${bind(JSON.stringify(some))}::jsonb))`;
      const hit = c.list ? `EXISTS (SELECT 1 FROM unnest(${col}) x WHERE to_jsonb(x) IN ${set})` : `to_jsonb(${col}) IN ${set}`;
      all.push(some.length < f.values.length ? `(${hit} OR ${col} IS NULL)` : hit);
    }
    if (f.contains) all.push(`${textOf(t, c, col)} ILIKE ${bind(like(f.contains))}`);
    const value = c.type === 'money' ? `(${col}).amount` : c.type === 'date' ? `${col}::date` : col;
    const cast = c.type === 'date' ? 'date' : 'numeric';
    if (f.min) all.push(`${value} >= ${bind(f.min)}::${cast}`);
    if (f.max) all.push(`${value} <= ${bind(f.max)}::${cast}`);
  }
  if (state.query.trim()) {
    const named = t.columns.filter((c) => c.name === 'name' || (shownOf(t).includes(c) && filterKind(c)));
    all.push(`concat_ws(' ', ${named.map((c) => textOf(t, c, `t.${quoted(c.name)}`)).join(', ')}) ILIKE ${bind(like(state.query.trim()))}`);
  }
  return { sql: all.length ? ` WHERE ${all.join(' AND ')}` : '', params };
}

function orderOf(t, state) {
  const id = t.columns.some((c) => c.name === 'id');
  const c = t.columns.find((x) => x.name === state.sort);
  if (!c) return id ? ' ORDER BY t.id' : '';
  const col = `t.${quoted(c.name)}`;
  const value = c.type === 'money' ? `(${col}).amount` : ['number', 'date'].includes(c.type) ? col : textOf(t, c, col);
  return ` ORDER BY ${value} ${c.type === 'date' ? 'DESC' : 'ASC'} NULLS LAST${id ? ', t.id' : ''}`;
}

const queryCode = (sql, params) => `archive.query(${[sql, ...params].map((p) => JSON.stringify(p)).join(', ')})`;

export const viewRows = resource({
  tables: (c) => !c.table.startsWith('archive.'),
  param: (c) => c.table,
  load: async (id) => {
    const v = ws();
    let asked;
    do {
      const state = viewState(id);
      asked = JSON.stringify(state);
      const t = tableOf(id, v) ?? (await schema.refresh(), tableOf(id, v));
      if (!t) return;
      const { sql, params } = whereOf(t, state);
      const from = ` FROM ${sourceOf(t)} t${sql}`;
      const fields = t.columns.map((c) => `t.${quoted(c.name)}`).join(', ');
      try {
        const got = await runCode(`return { rows: await ${queryCode(`SELECT ${fields}${from}${orderOf(t, state)} LIMIT ${VIEW_PAGE} OFFSET ${(state.page - 1) * VIEW_PAGE}`, params)}, total: (await ${queryCode(`SELECT count(*)::int AS n${from}`, params)})[0].n };`, v.id);
        if (!got.rows.length && got.total) views[id] = { ...state, page: Math.ceil(got.total / VIEW_PAGE) };
        v.viewRows = { ...v.viewRows, [id]: got.rows };
        v.viewTotals = { ...v.viewTotals, [id]: { total: got.total, error: '' } };
      } catch (e) {
        if (e instanceof Unauthorized) throw e;
        v.viewRows = { ...v.viewRows, [id]: [] };
        v.viewTotals = { ...v.viewTotals, [id]: { total: 0, error: e.message } };
      }
    } while (asked !== JSON.stringify(viewState(id)));
  }
});

let viewTimer = null;

export function setView(id, patch, wait = 0) {
  views[id] = { ...viewState(id), page: 1, ...patch };
  clearTimeout(viewTimer);
  viewTimer = setTimeout(() => viewRows.refresh(id), wait);
}

export function setFilter(id, name, filter, wait = 0) {
  const filters = { ...viewState(id).filters, [name]: filter };
  if (!filter?.values?.length && !filter?.contains && !filter?.min && !filter?.max) delete filters[name];
  setView(id, { filters }, wait);
}

export async function filterOptions(id, c, term) {
  const t = tableOf(id);
  const { sql, params } = whereOf(t, viewState(id), c.name);
  const col = `t.${quoted(c.name)}`;
  const source = c.list ? `unnest(${col}) v` : `LATERAL (SELECT ${col} AS v) v`;
  const search = term ? `${sql ? ' AND' : ' WHERE'} ${textOf(t, c, 'v.v')} ILIKE $${params.push(like(term))}` : '';
  return (await runCode(`return await ${queryCode(`SELECT to_jsonb(v.v) AS v, count(*)::int AS n FROM ${sourceOf(t)} t, ${source}${sql}${search} GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 100`, params)};`)) ?? [];
}

export function tableOf(id, v = ws()) {
  return v?.schema.find((x) => x.id === id) ?? v?.archiveTables?.find((x) => x.id === id);
}

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

export function linkOf(t, c, depth = 0) {
  if (c.to || t?.kind !== 'view' || c.type !== 'id' || depth > 4) return c.to;
  const base = tableOf(baseOf(t));
  const col = base !== t && base?.columns.find((x) => x.name === c.name);
  return col ? linkOf(base, col, depth + 1) : undefined;
}

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

export async function rowOf(table, id) {
  return call(`/api/${encodeURIComponent(table)}/${encodeURIComponent(id)}`);
}

export function titleOf(row, t) {
  const own = show(row?.name).trim();
  if (own || t?.kind !== 'view') return own || 'Untitled';
  const base = baseOf(t);
  return (base !== t.id && tableOf(base) && nameOf(base, row?.id)?.trim()) || 'Untitled';
}

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

onChange((changes, sweep) => {
  for (const v of app.archives) {
    for (const table of Object.keys(v.names ?? {})) {
      if (sweep || changes.some((c) => c.table === table || c.table === `archive.${table}`)) {
        for (const id of Object.keys(v.names[table])) ask(v, table, id);
      }
    }
  }
});

export function nameOf(table, id, v = ws()) {
  if (!v || !table || typeof id !== 'string' || !UUID.test(id)) return null;
  const got = v.names?.[table]?.[id];
  if (got === undefined && !reading.has(`${v.id}\n${table}\n${id}`)) ask(v, table, id);
  return got;
}

export function linkTitle(table, id) {
  const name = nameOf(table, id);
  return name === undefined ? '…' : name === null ? 'Not found' : name.trim() || 'Untitled';
}

export function linkHref(table, id) {
  return table === 'archive.file' ? `/library/${id}` : `/view/${table}/${id}`;
}

export function cellText(t, c, row) {
  const v = row?.[c.name];
  const to = linkOf(t, c);
  if (!to || v == null || v === '') return show(v);
  return (Array.isArray(v) ? v : [v]).map((id) => linkTitle(to, id)).join(', ');
}

export async function rowRelations(id, row) {
  const v = ws();
  const t = tableOf(id, v);
  if (!t || !row?.id) return { from: [] };
  const base = baseOf(t);
  const all = v.catalog.catalog.tables;
  const target = `"${all[base].schema}"."${base}"`;
  const uses = (await runCode(`return await archive.query("select * from archive.uses($1::regclass, $2)", ${JSON.stringify(target)}, ${JSON.stringify(row.id)});`)) ?? [];
  return {
    from: uses
      .filter((u) => all[u.table]?.schema === u.schema)
      .map((u) => ({ table: u.table, column: u.column, rows: u.rows.map((r) => ({ id: r.id, title: titleOf(r) })) }))
  };
}

export function clearUnread(v) {
  v.unread = false;
  api('/auth/unread', { method: 'DELETE', workspace: v.id }).catch(() => {});
}

function prepare(v, booting = api('/api/boot', { workspace: v.id })) {
  v.boot = booting;
  v.boot.then((data) => v.id === app.archive && Object.assign(app.signup, data.signup)).catch(() => {});
}

async function loadArchive(v = ws()) {
  if (v.unread) clearUnread(v);
  v.loading = true;
  await Promise.allSettled([agents.refresh(), members.refresh(), workflows.refresh(), schema.refresh()]);
  v.boot = null;
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

export function openSettings(section = null, sub = null) {
  Object.assign(app.settings, { open: true, section, sub });
}

export const onboards = () => ws()?.look.type === 'business';

export const onboardingSteps = [
  { id: 'msa', group: 'Legal', title: 'Sign the MSA and DPA', body: 'The terms for your archive, billing, support and personal data.', action: 'Review and sign' },
  { id: 'baa', group: 'Legal', title: 'Sign the BAA', body: 'How we protect health information on your behalf.', action: 'Review and sign' },
  { id: 'business', group: 'Billing', title: 'Add your business details', body: 'Your legal name, EIN and address.', action: 'Add' },
  { id: 'card', group: 'Billing', title: 'Link a card', body: 'We charge it under your agreement.', action: 'Link' }
];

export async function loadSignup() {
  try {
    Object.assign(app.signup, (await call('/api/boot?only=signup')).signup);
  } catch (e) {
    await complain(e);
  }
}

export const signed = (kind) => app.signup.agreement.find((a) => a.kind === kind);

export function stepDone(id) {
  if (id === 'business') return app.signup.business.length > 0;
  if (id === 'msa' || id === 'baa') return Boolean(signed(id));
  return Boolean(app.signup.billing[0]?.name);
}

export async function saveBusiness(data) {
  const id = app.signup.business[0]?.id;
  await call(id ? `/api/business/${id}` : '/api/business', { method: id ? 'PATCH' : 'POST', body: data });
  await loadSignup();
}

export async function signAgreement(data) {
  await call('/api/agreement', { method: 'POST', body: data });
  await loadSignup();
}

export const startCard = () => call('/api/billing', { method: 'POST', body: {} });

export async function cardLinked(setup_intent) {
  await call('/api/billing', { method: 'POST', body: { setup_intent } });
  await loadSignup();
}

export function seedOf(person) {
  const p = typeof person === 'string' ? { name: person } : person;
  if (p.user || p.id) return p.user ?? p.id;
  const m = ws()?.members.find((x) => (p.email && x.email === p.email) || (!p.email && x.name === p.name));
  return m?.user ?? m?.id ?? p.email ?? p.name ?? '';
}

export const ROLES = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'editor', label: 'Editor' },
  { value: 'manager', label: 'Manager' },
  { value: 'engineer', label: 'Engineer' },
  { value: 'owner', label: 'Owner' }
];

const rank = (role) => ROLES.findIndex((r) => r.value === role);

export function atLeast(role, of = app.role) {
  return rank(of) >= rank(role);
}

export async function invite({ emails, role, group }) {
  const w = ws();
  const added = [];
  for (const email of emails) {
    if (w.members.some((m) => m.email === email)) continue;
    try {
      const m = memberOf(await call('/api/member', { method: 'POST', body: { email, role, group } }));
      w.members = [...w.members, m].sort((a, b) => a.email.localeCompare(b.email));
      added.push(m);
    } catch (e) {
      await complain(e, `Couldn’t invite ${email}`);
    }
  }
  return added;
}

export async function removeMember(id) {
  const w = ws();
  const m = w.members.find((x) => x.id === id);
  if (!m) return;
  try {
    await call(`/api/member/${m.id}`, { method: 'DELETE' });
    w.members = w.members.filter((x) => x.id !== id);
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


export function groupById(id, w = ws()) {
  return w?.groups.find((g) => g.id === id);
}

export async function setGroup(member, group) {
  const before = member.group;
  member.group = group;
  try {
    await call(`/api/member/${member.id}`, { method: 'PATCH', body: { group } });
    return true;
  } catch (e) {
    member.group = before;
    complain(e, `Couldn’t change ${member.name}’s group`);
    return false;
  }
}

export function mayLimit(current) {
  if (atLeast('manager')) return true;
  const me = ws().members.find((m) => m.email === app.user.email);
  return app.role === 'editor' && (!current || me?.group === current);
}

export function accessOptions(current) {
  const me = ws().members.find((m) => m.email === app.user.email);
  const mine = ws().groups.filter((g) => atLeast('manager') || me?.group === g.id || g.id === current);
  return [{ value: '', label: 'Everyone' }, ...mine.map((g) => ({ value: g.id, label: g.name }))];
}


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
  for (const m of w.members) if (people.includes(m.id)) await setGroup(m, group.id);
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
  for (const m of w.members) if (m.group === group.id) m.group = null;
  return true;
}

export async function groupUses(id) {
  const sql = `select "table", total from archive.uses('archive."group"', $1) where restricts and "table" <> 'group'`;
  const rows = (await runCode(`return await archive.query(${JSON.stringify(sql)}, ${JSON.stringify(id)});`)) ?? [];
  return rows.map((r) => ({ table: r.table, rows: Number(r.total) }));
}

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


export function archiveIntegrations() {
  return ws().integrations ?? [];
}

export function connectIntegration(id, asked) {
  if (atLeast('manager')) connect(id, asked);
}

async function onIntegration(id, what, act) {
  const c = archiveIntegrations().find((x) => x.id === id);
  if (!c?.connection || !atLeast('manager')) return;
  try {
    await act(c.connection.id);
  } catch (e) {
    complain(e, `Couldn’t ${what} ${c.name}`);
  }
  return connections.refresh();
}

export const disconnectIntegration = (id) => onIntegration(id, 'disconnect', (c) => call(`/api/connection/${c}`, { method: 'DELETE' }));
export const syncIntegration = (id) => onIntegration(id, 'sync', (c) => call(`/api/connection/${c}`, { method: 'PATCH', body: { sync: true } }));


const KEY_NAME = 'Archive API key';

export function archiveKey() {
  return app.archiveKeys[app.archive] ?? null;
}

const personalKey = (x) => x.name === KEY_NAME && !x.app && !x.workspace;
const workspaceKey = (id) => (x) => x.name === KEY_NAME && !x.app && x.workspace === id;
const newest = (list) => [...list].sort((a, b) => b.created_at.localeCompare(a.created_at));

async function withSecret(kept, t) {
  if (kept?.secret && kept.id === t?.id) return kept;
  if (!t) return null;
  const secret = (await call(`/auth/tokens/${t.id}/secret`).catch(() => null))?.secret ?? '';
  return { id: t.id, secret };
}

export const tokens = resource({
  tables: ['archive.token'],
  load: async () => {
    const id = app.archive;
    const list = (await call('/auth/tokens')) ?? [];
    app.apps = newest(list.filter((x) => x.app));
    app.apiKey = await withSecret(app.apiKey, newest(list.filter(personalKey))[0]);
    app.archiveKeys[id] = await withSecret(app.archiveKeys[id], newest(list.filter(workspaceKey(id)))[0]);
  }
});

export async function revokeApp(id) {
  const before = app.apps;
  app.apps = before.filter((x) => x.id !== id);
  try {
    await call(`/auth/tokens/${id}`, { method: 'DELETE' });
  } catch (e) {
    app.apps = before;
    complain(e, 'Couldn’t disconnect it');
  }
}

export async function rotateKey(personal = false) {
  if (!personal && !atLeast('engineer')) return;
  const id = app.archive;
  try {
    const old = (await call('/auth/tokens')).filter(personal ? personalKey : workspaceKey(id));
    const t = await call('/auth/tokens', { method: 'POST', body: { name: KEY_NAME, personal } });
    for (const o of old) await call(`/auth/tokens/${o.id}`, { method: 'DELETE' }).catch(() => {});
    const key = { id: t.id, secret: t.token };
    if (personal) app.apiKey = key;
    else app.archiveKeys[id] = key;
  } catch (e) {
    complain(e, 'Couldn’t make a new key');
  }
}

function isNarrow() {
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

export function toggleDeveloper() {
  app.developer = !app.developer;
  localStorage.setItem('archive.developer', app.developer ? 'on' : 'off');
}

function removeAt(list, predicate, after) {
  const i = list.findIndex(predicate);
  if (i < 0) return null;
  const [item] = list.splice(i, 1);
  after?.(item);
  return item;
}

export async function switchArchive(id) {
  if (id === app.archive || !app.archives.some((v) => v.id === id)) return;
  setWorkspace(id);
  app.archive = id;
  app.signup = noSignup();
  const head = route.parts[0];
  if (['agent', 'workflows', 'database'].includes(head) && route.parts[1]) go(`/${head === 'agent' ? '' : head}`);
  prepare(ws());
  listen(id, ws().boot.then((data) => data.live));
  loadArchive(ws());
}

export async function createArchive(name, look) {
  if (!app.canAdd) {
    await notify({ title: 'Ask an admin for a new archive', body: 'Only the people who run this Archive can make archives.' });
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

export async function saveArchive({ name, look }) {
  const w = ws();
  const before = { name: w.name, look: w.look, kind: w.kind };
  Object.assign(w, { name: name || w.name, look: { ...look }, kind: kindOf(look.type) });
  try {
    const saved = await call('/auth/workspace', { method: 'PATCH', body: { name: w.name, ...lookBody(look) } });
    Object.assign(w, { name: saved.name, slug: saved.slug, look: lookOf(saved), kind: kindOf(saved.type) });
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

export function agentById(id) {
  return ws()?.agents.find((a) => a.id === id);
}

export function agentAddress(agent) {
  return `${ws().slug}-${agent.slug}@${location.hostname.replace(/^app\./, '')}`;
}

export function rememberAgent(id) {
  setLocal(`archive.agent:${app.archive}`, id);
}

export function lastAgent() {
  return agentById(local(`archive.agent:${app.archive}`, null)) ?? ws()?.agents[0];
}

export function askAgent(text, attachments = []) {
  const agent = lastAgent();
  if (!agent) return '/';
  app.pending = { id: agent.id, text, attachments };
  return `/agent/${agent.id}`;
}

export const unread = (agent) => agent.at !== agent.seen && !agent.streaming;

export function markSeen(agent) {
  agent.seen = agent.at;
  setLocal(`archive.seen:${app.archive}`, { ...local(`archive.seen:${app.archive}`, {}), [agent.id]: agent.at });
}

export async function createAgent(row) {
  try {
    const agent = await call('/api/agent', { method: 'POST', body: row });
    ws().agents.push({ ...agent, at: '', seen: '' });
    go(`/agent/${agent.id}`);
  } catch (e) {
    complain(e, 'Couldn’t create the agent');
  }
}

export async function updateAgent(id, patch) {
  const agent = agentById(id);
  if (!agent) return;
  const before = { ...agent };
  Object.assign(agent, patch);
  try {
    const saved = await call(`/api/agent/${id}`, { method: 'PATCH', body: patch });
    agent.slug = saved.slug;
  } catch (e) {
    Object.assign(agent, before);
    complain(e, 'Couldn’t change the agent');
  }
}

export async function removeAgent(id) {
  const w = ws();
  try {
    await call(`/api/agent/${id}`, { method: 'DELETE' });
  } catch (e) {
    return complain(e, 'Couldn’t delete the agent');
  }
  saveDraft(id, '');
  w.agents = w.agents.filter((a) => a.id !== id);
  delete w.conversations[id];
  go('/');
}

export async function uploadFiles(list, parent) {
  const w = ws();
  const added = [];
  for (const file of list) {
    try {
      const f = filed(toFile(await upload(file, undefined, parent), w.name), w);
      if (!parent) w.library.unshift(f);
      added.push(f);
    } catch (e) {
      await complain(e, `Couldn’t upload ${file.name}`);
    }
  }
  return added;
}

function filed(f, w) {
  f.folder = f.access ? (groupById(f.access, w)?.name ?? 'Unknown group') : f.connection ? 'Who sees its connection' : 'Everyone';
  if (f.kind === 'folder' && f.access) f.sub = f.folder;
  return f;
}

export async function setFileAccess(file, access) {
  const w = ws();
  try {
    const f = filed(toFile(await call(`/api/file/${file.id}`, { method: 'PATCH', body: { access } }), w.name), w);
    Object.assign(file, { access: f.access, folder: f.folder, sub: f.sub });
  } catch (e) {
    complain(e, `Couldn’t change who sees ${file.name}`);
  }
}

export async function addFolder(name, parent) {
  const w = ws();
  try {
    return filed(toFile(await call('/api/file', { method: 'POST', body: { kind: 'folder', name, parent: parent ?? null } }), w.name), w);
  } catch (e) {
    complain(e, `Couldn’t make ${name}`);
  }
}

export async function renameFile(file, name) {
  if (!name) return;
  const before = file.name;
  file.name = name;
  try {
    await call(`/api/file/${file.id}`, { method: 'PATCH', body: { name } });
  } catch (e) {
    file.name = before;
    complain(e, 'Couldn’t rename the file');
  }
}

export async function moveFile(file, parent) {
  try {
    await call(`/api/file/${file.id}`, { method: 'PATCH', body: { parent } });
  } catch (e) {
    complain(e, `Couldn’t move ${file.name}`);
  }
}

export async function removeFile(id) {
  const w = ws();
  try {
    await call(`/api/file/${id}`, { method: 'DELETE' });
    removeAt(w.library, (f) => f.id === id);
  } catch (e) {
    complain(e, 'Couldn’t delete the file');
  }
}

export async function downloadFile(file) {
  try {
    const link = await call(`/api/file/${file.id}/link?expires=5m`);
    Object.assign(document.createElement('a'), { href: `${link.url}&download=1` }).click();
  } catch (e) {
    complain(e, `Couldn’t download ${file.name}`);
  }
}

export async function downloadAgreement(row) {
  try {
    const { doc } = await call(`/api/agreement/${row.id}`);
    const bytes = Uint8Array.from(doc.slice(2).match(/../g), (h) => parseInt(h, 16));
    const href = URL.createObjectURL(new Blob([bytes], { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' }));
    Object.assign(document.createElement('a'), { href, download: `${row.name} - ${row.company}.docx` }).click();
    setTimeout(() => URL.revokeObjectURL(href), 60000);
  } catch (e) {
    complain(e, `Couldn’t download ${row.name}`);
  }
}

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

export async function renameView(id, name) {
  const to = name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
  if (!to || to === id || !atLeast('manager')) return;
  if (!/^[a-z]/.test(to)) return complain(new Error('A view’s name starts with a letter.'), 'Couldn’t rename the view');
  const sql = `ALTER VIEW public."${id}" RENAME TO "${to}"`;
  try {
    const code = `const dry = await archive.migrate(${JSON.stringify(sql)}); return archive.migrate(${JSON.stringify(sql)}, { confirm: dry.confirm });`;
    const { error } = await call('/api/run', { method: 'POST', body: { code } });
    if (error) throw new Error(error);
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

export const connections = resource({
  tables: ['archive.connection'],
  load: async () => {
    const v = ws();
    if (!v.catalog) await schema.refresh();
    const connectors = v.catalog.catalog.connectors;
    const rows = await api('/api/connection?limit=1000', { workspace: v.id });
    rows.sort((a, b) => a.provider.localeCompare(b.provider) || a.account.localeCompare(b.account));
    const ids = new Set(connectors.filter((k) => !k.integration).map((k) => k.id));
    v.connectors = v.connectors.filter((c) => ids.has(c.id));
    nameBrands(connectors);
    v.integrations = connectors.filter((k) => k.integration).map((k) => toIntegration(k, rows.find((x) => x.provider === k.id && x.integration)));
    for (const k of connectors.filter((k) => !k.integration)) {
      let c = v.connectors.find((x) => x.id === k.id);
      if (!c) {
        c = { by: k.name, description: `Use your ${k.name} account`, category: 'Other', scopes: [], ...k };
        v.connectors.push(c);
        c = v.connectors.at(-1);
      }
      const mine = rows.filter((x) => x.provider === k.id && !x.integration);
      const synced = mine.every((x) => x.synced_at) ? mine.map((x) => x.synced_at).sort()[0] : '';
      Object.assign(c, {
        ready: k.ready,
        keys: k.keys ?? [],
        ask: k.ask ?? null,
        link: !!k.link,
        tap: !!k.tap,
        connections: mine,
        connected: mine.length > 0,
        account: mine.map((x) => x.account).join(', '),
        error: mine.find((x) => x.error)?.error ?? '',
        lastSync: synced ? relative(synced) : mine.some((x) => x.sync_at) ? 'Syncing…' : 'Not yet',
        items: mine.reduce((n, x) => n + (x.items ?? 0), 0)
      });
    }
  }
});

function lastSyncOf(x) {
  return x.synced_at ? relative(x.synced_at) : x.sync_at ? 'Syncing…' : 'Not yet';
}

function toIntegration(k, x) {
  const shown = { by: k.name, description: `Use the archive’s ${k.name} account`, category: 'Other', scopes: [], ...k };
  return {
    ...shown,
    id: k.id,
    name: k.name,
    ready: k.ready,
    keys: k.keys ?? [],
    ask: k.ask ?? null,
    link: !!k.link,
    tap: !!k.tap,
    connected: Boolean(x),
    connection: x ?? null,
    account: x?.account ?? '',
    member: x?.member ?? '',
    date: x ? relative(x.created_at) : '',
    lastSync: x ? lastSyncOf(x) : '',
    items: x?.items ?? 0,
    error: x?.error ?? '',
    access: x?.access ?? ''
  };
}

export function connect(id, asked = {}, workspace = app.archive) {
  const c = [...ws().connectors, ...archiveIntegrations()].find((x) => x.id === id);
  const q = new URLSearchParams({ ...asked, workspace });
  if (c?.ready && c.tap) connectKeys(id, {}, workspace);
  else if (c?.ready) location.assign(`${API}/connect/${encodeURIComponent(id)}?${q}`);
}

export async function connectPlaces(id, q) {
  return (await call(`/api/search?provider=${encodeURIComponent(id)}&q=${encodeURIComponent(q)}`)) ?? [];
}

export async function connectKeys(id, keys, workspace, quiet = false) {
  try {
    await call('/api/connection', { method: 'POST', body: { provider: id, keys }, workspace });
  } catch (e) {
    if (quiet && !(e instanceof Unauthorized)) return e;
    complain(e, 'Couldn’t connect');
    return false;
  }
  await connections.refresh();
  return true;
}

const LINK_SCRIPT = 'https://cdn.plaid.com/link/v2/stable/link-initialize.js';
const LINKING = 'archive.linking';
let linkScript;

function loadLink() {
  linkScript ??= new Promise((resolve, reject) => {
    const s = Object.assign(document.createElement('script'), { src: LINK_SCRIPT, onload: () => resolve(window.Plaid) });
    s.onerror = () => {
      linkScript = null;
      reject(new Error('Plaid Link didn’t load. Check your connection and try again.'));
    };
    document.head.append(s);
  });
  return linkScript;
}

function linking(v) {
  try {
    if (v === undefined) return JSON.parse(sessionStorage.getItem(LINKING) ?? 'null');
    if (v) sessionStorage.setItem(LINKING, JSON.stringify(v));
    else sessionStorage.removeItem(LINKING);
  } catch {
    return null;
  }
}

async function openLink({ id, token, workspace }, back) {
  const Plaid = await loadLink();
  Plaid.create({
    token,
    receivedRedirectUri: back,
    onSuccess: (publicToken) => {
      linking(null);
      connectKeys(id, { public_token: publicToken }, workspace);
    },
    onExit: (err) => {
      linking(null);
      if (err) notify({ title: 'That didn’t connect', body: err.display_message || err.error_message || err.error_code });
    }
  }).open();
}

export async function connectLink(id) {
  try {
    const { token } = await call('/api/connection', { method: 'POST', body: { provider: id } });
    const at = { id, token, workspace: app.archive };
    linking(at);
    await openLink(at);
  } catch (e) {
    linking(null);
    complain(e, 'Couldn’t connect');
  }
}

function resumeLink() {
  if (!new URLSearchParams(location.search).has('oauth_state_id')) return;
  const at = linking();
  const back = location.href;
  history.replaceState(null, '', location.pathname);
  if (at) openLink(at, back).catch((e) => complain(e, 'Couldn’t connect'));
}

export const ownConnections = resource({
  tables: ['archive.connection'],
  load: async () => {
    const all = await Promise.all(app.archives.map(async (v) => {
      const rows = await api('/api/connection?integration=false&limit=1000', { workspace: v.id });
      return rows.map((x) => ({ ...x, workspace: { id: v.id, name: v.name }, look: v.look }));
    }));
    app.ownConnections = all.flat().sort((a, b) => a.provider.localeCompare(b.provider) || a.account.localeCompare(b.account) || a.workspace.name.localeCompare(b.workspace.name));
  }
});

async function onOwnConnection(x, act, failed) {
  try {
    await act(x.workspace.id);
  } catch (e) {
    complain(e, failed);
  }
  if (x.workspace.id === app.archive) connections.refresh();
  return ownConnections.refresh();
}

export const syncOwnConnection = (x) => onOwnConnection(x, (workspace) => call(`/api/connection/${x.id}`, { method: 'PATCH', body: { sync: true }, workspace }), `Couldn’t sync ${x.account}`);
export const disconnectOwnConnection = (x) => onOwnConnection(x, (workspace) => call(`/api/connection/${x.id}`, { method: 'DELETE', workspace }), `Couldn’t disconnect ${x.account}`);

