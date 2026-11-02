// The app's state. An archive is one Archive workspace: its chats, files, tables, members and workflows come from
// the API (see live.js for the mapping). What the API cannot do yet comes from mock/ and stays in this tab or
// in localStorage; mock/index.js lists those pieces.

import { api, blobURL, forgetBlobs, upload, setWorkspace, workspace, local, setLocal, signOut as apiSignOut, Unauthorized, passkeys, passkeyAsked } from './api.js';
import { resource, listen, onChange, onRefused } from './stream.svelte.js';
import { toFile, toChat, toMember, toWorkflow, toRun } from './live.js';
import { toTables, countsQuery } from './schema.js';
import { initials, mockConnectors, mockIntegrations, mockKey, mockProfile, mockSettings } from './mock/index.js';
import { integrationCatalog } from './integrations.js';
import { defaultLook, TYPES } from './looks.js';
import { notify } from './dialogs.svelte.js';
import { go, route } from './router.svelte.js';

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
  integrations: mockIntegrations(),
  apiKey: mockKey('ak_live_', 'Created today'),
  archiveKeys: {},
  share: { open: false, kind: null, target: null },
  onboarding: [],
  profile: mockProfile()
});

const kindOf = (type) => ({ family: 'Family', business: 'Business', personal: 'Just you' })[type] ?? 'Archive';

// An archive's look is its workspace's type, icon and colour, from the API; without an icon or a colour it has
// its type's.
function lookOf(w) {
  const type = TYPES[w.type] ? w.type : 'personal';
  const d = defaultLook(type);
  return { type, icon: w.icon ?? d.icon, tint: w.color ?? d.tint };
}

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
    schema: [],
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

export const currentArchive = ws;

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
  const named = new URLSearchParams(location.search).get('workspace');
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
  canAdd();
}

function canAdd() {
  api('/auth/workspaces', { workspace: app.archives[0]?.id })
    .then(() => (app.canAdd = true))
    .catch(() => (app.canAdd = false));
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
  canAdd();
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

// The library's files, and its folders: every access group a file is in, and the empty ones made here.
export const library = resource({
  tables: ['archive.file'],
  load: async () => {
    const v = ws();
    const list = await files(v.id);
    v.library = list.map((f) => toFile(f, v.name));
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

export async function loadArchive(v = ws()) {
  v.loading = true;
  await Promise.allSettled([chats.refresh(), library.refresh(), members.refresh(), workflows.refresh(), schema.refresh()]);
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
  { id: 'connect', group: 'Your data', title: 'Connect a source', body: 'Bring in files from Google Drive, Dropbox, email or iCloud Photos.', action: 'Connect' },
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

function archiveDomain() {
  return (app.user.email ?? '').split('@')[1] ?? '';
}

export async function invite({ emails, role }) {
  const w = ws();
  const added = [];
  for (const email of emails) {
    if (w.members.some((m) => m.email === email)) continue;
    try {
      const people = await call('/auth/users', { method: 'POST', body: { email, role } });
      w.members = people.map((p) => toMember(p, { ...app.user, user_id: app.user.id }));
      added.push(w.members.find((m) => m.email === email));
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

// Integrations (MOCK: SSO, SCIM and workspace apps have no API yet)

export function archiveIntegrations() {
  const active = ws().members.filter((m) => m.status === 'active');
  return integrationCatalog.map((c) => {
    const link = app.integrations.find((x) => x.archive === app.archive && x.id === c.id);
    const domain = c.covers === 'domain' ? link?.account : null;
    const members = link ? active.filter((m) => !domain || m.email.endsWith(`@${domain}`)) : [];
    return { ...c, connected: Boolean(link), account: link?.account ?? '', connectedBy: link?.by ?? '', date: link?.date ?? '', members };
  });
}

export function connectIntegration(id) {
  const c = integrationCatalog.find((x) => x.id === id);
  if (!c || !isArchiveOwner() || app.integrations.some((x) => x.archive === app.archive && x.id === id)) return;
  const domain = archiveDomain();
  const account = c.id === 'slack' ? `${domain.split('.')[0]}.slack.com` : c.covers === 'domain' ? domain : ws().name;
  app.integrations.push({ archive: app.archive, id, account, by: app.profile.name, date: 'Today' });
}

export function disconnectIntegration(id) {
  const i = app.integrations.findIndex((x) => x.archive === app.archive && x.id === id);
  if (i === -1 || !isArchiveOwner()) return;
  app.integrations.splice(i, 1);
}

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
    const t = list.filter((x) => x.name === KEY_NAME).sort((a, b) => b.created_at.localeCompare(a.created_at))[0];
    if (app.archiveKeys[id]?.secret && app.archiveKeys[id].id === t?.id) return;
    app.archiveKeys[id] = t ? { id: t.id, secret: '', date: `Created ${new Date(t.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}` } : null;
  }
});

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
  const groups = new Set(w.members.flatMap((m) => m.groups));
  const access = folder && groups.has(folder) ? folder : undefined;
  const added = [];
  for (const file of list) {
    try {
      const f = toFile(await upload(file, access), w.name);
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

// Folders are the access group a file is limited to ("Everyone" when it isn't). A new, empty one is MOCK
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

export function toggleWorkflow(id) {
  const wf = workflowById(id);
  if (wf) wf.enabled = !wf.enabled;
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

export function pauseRun() {}
export function stopRun() {}
export function resolveRun() {}
export function steerRun() {}

// Connectors (MOCK: per-person app connections have no API yet; kept in this tab)

export function connect(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  c.connected = true;
  c.account = app.user.email;
  c.lastSync = 'Just now';
  c.items = 0;
  completeStep('connect');
}

export function disconnect(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  c.connected = false;
  c.account = '';
  c.lastSync = '';
}

export function syncConnector(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  c.lastSync = 'Just now';
  c.items += 3;
}

export { initials };
