import { me, initials } from './data.js';
import { seed, blankVault } from './seed.js';
import { integrationCatalog } from './integrations.js';
import { go, route } from './router.svelte.js';

let counter = 0;
const nid = (prefix) => `${prefix}${Date.now().toString(36)}${(counter++).toString(36)}`;

export const app = $state({
  vaults: seed(),
  vault: 'okafor',
  collapsed: typeof localStorage !== 'undefined' && localStorage.getItem('archive.sidebar') === 'collapsed',
  drawer: false,
  details: false,
  palette: false,
  help: false,
  model: 'pro',
  pending: null,
  signedOut: false,
  muted: false,
  settings: { open: false, section: null, sub: null },
  vaultSettings: { open: false, section: null, sub: null },
  integrations: [
    { vault: 'okafor', id: 'google-workspace', account: 'okafor.family', by: 'Nora Okafor', date: 'Sep 12' },
    { vault: 'okafor', id: 'google-sso', account: 'okafor.family', by: 'Emeka Okafor', date: 'Sep 12' },
    { vault: 'hale', id: 'microsoft-365', account: 'halewhitcombe.law', by: 'Ruth Hale', date: 'Mar 3' },
    { vault: 'hale', id: 'entra', account: 'halewhitcombe.law', by: 'Daniel Whitcombe', date: 'Mar 3' },
    { vault: 'hale', id: 'scim', account: 'Entra ID', by: 'Daniel Whitcombe', date: 'Mar 4' },
    { vault: 'hale', id: 'box', account: 'Hale & Whitcombe', by: 'Ruth Hale', date: 'Apr 18' },
    { vault: 'fieldnote', id: 'google-workspace', account: 'fieldnote.studio', by: 'Jun Park', date: 'Jan 9' },
    { vault: 'fieldnote', id: 'slack', account: 'fieldnote.slack.com', by: 'Jun Park', date: 'Jan 9' },
    { vault: 'fieldnote', id: 'notion', account: 'Fieldnote', by: 'Lea Fontaine', date: 'Feb 21' }
  ],
  apiKey: { secret: 'ak_live_4f9c2e71b0d8a63e5c1f7b294d06a1b2', date: 'Created Sep 14' },
  vaultKeys: {
    okafor: { secret: 'ak_vault_8e1d47c3a95f02b6d7e43c18f2a97f3c', date: 'Created Aug 2' },
    hale: { secret: 'ak_vault_2b7f19e04c6d83a5f1e9b72d40c3e90d', date: 'Rotated Oct 3' },
    fieldnote: { secret: 'ak_vault_c03a8d5e7f1b29e64a0d5c87b3f15d21', date: 'Created Jan 9' },
    personal: { secret: 'ak_vault_71e5b0c9d2a48f36e1b07d95c2a84b6e', date: 'Created Feb 1' }
  },
  share: { open: false, kind: null, target: null },
  onboarding: ['verify', 'invite', 'upload'],
  profile: { name: me.name, first: me.first, work: 'archivist', prefs: 'Keep answers short. Use British spelling. When you quote a letter, keep the original spelling and punctuation.', tone: me.tone, notify: { done: true, mentions: true, digest: true, invites: false } }
});

export const onboardingSteps = [
  { id: 'msa', group: 'Legal', title: 'Sign the master services agreement', body: 'The terms for your vault, billing and support.', action: 'Review and sign' },
  { id: 'dpa', group: 'Legal', title: 'Sign the data processing agreement', body: 'How we process personal data on your behalf under GDPR.', action: 'Review and sign' },
  { id: 'verify', group: 'Legal', title: 'Verify your business details', body: 'Confirm your legal name, address and VAT number for invoices.', action: 'Verify' },
  { id: 'invite', group: 'Your team', title: 'Invite your team', body: 'Add the people who should read, edit or own this vault.', action: 'Invite' },
  { id: 'connect', group: 'Your data', title: 'Connect a source', body: 'Bring in files from Google Drive, Dropbox, email or iCloud Photos.', action: 'Connect' },
  { id: 'upload', group: 'Your data', title: 'Upload your first files', body: 'Drop in scans, letters, recordings or spreadsheets.', action: 'Upload' },
  { id: 'workflow', group: 'Your data', title: 'Set up a workflow', body: 'Let an agent file, tag or transcribe new items as they arrive.', action: 'Create' },
  { id: 'region', group: 'Your data', title: 'Choose a data region and retention', body: 'Pick where your vault lives and how long deleted items are kept.', action: 'Choose' }
];

export function stepDone(id) {
  return app.onboarding.includes(id);
}

export function completeStep(id) {
  if (stepDone(id)) return;
  app.onboarding.push(id);
}

export function toggleStep(id) {
  const i = app.onboarding.indexOf(id);
  if (i === -1) app.onboarding.push(id);
  else app.onboarding.splice(i, 1);
}

export function seedOf(person) {
  const p = typeof person === 'string' ? { name: person } : person;
  if (p.user || p.id) return p.user ?? p.id;
  const m = ws().members.find((x) => (p.email && x.email === p.email) || (!p.email && x.name === p.name));
  return m?.user ?? m?.id ?? p.email ?? p.name ?? '';
}

export function ws() {
  return app.vaults.find((w) => w.id === app.vault) ?? app.vaults[0];
}

export const currentVault = ws;

export function openSettings(section = null, sub = null) {
  app.vaultSettings.open = false;
  Object.assign(app.settings, { open: true, section, sub });
}

export function openVaultSettings(section = null, sub = null) {
  app.settings.open = false;
  Object.assign(app.vaultSettings, { open: true, section, sub });
}

function vaultMember() {
  return ws().members.find((m) => m.user === me.id || m.name === app.profile.name);
}

export function isVaultOwner() {
  return vaultMember()?.role === 'owner';
}

function vaultDomain() {
  return (vaultMember()?.email ?? '').split('@')[1] ?? '';
}

export function vaultIntegrations() {
  const active = ws().members.filter((m) => m.status === 'active');
  return integrationCatalog.map((c) => {
    const link = app.integrations.find((x) => x.vault === app.vault && x.id === c.id);
    const domain = c.covers === 'domain' ? link?.account : null;
    const members = link ? active.filter((m) => !domain || m.email.endsWith(`@${domain}`)) : [];
    return { ...c, connected: Boolean(link), account: link?.account ?? '', connectedBy: link?.by ?? '', date: link?.date ?? '', members };
  });
}

export function connectIntegration(id) {
  const c = integrationCatalog.find((x) => x.id === id);
  if (!c || !isVaultOwner() || app.integrations.some((x) => x.vault === app.vault && x.id === id)) return;
  const domain = vaultDomain();
  const account = c.id === 'slack' ? `${domain.split('.')[0]}.slack.com` : c.covers === 'domain' ? domain : ws().name;
  app.integrations.push({ vault: app.vault, id, account, by: app.profile.name, date: 'Today' });
}

export function disconnectIntegration(id) {
  const i = app.integrations.findIndex((x) => x.vault === app.vault && x.id === id);
  if (i === -1 || !isVaultOwner()) return;
  app.integrations.splice(i, 1);
}

function mockKey(prefix) {
  const hex = () => Math.random().toString(16).slice(2, 10).padEnd(8, '0');
  return `${prefix}${hex()}${hex()}${hex()}${hex()}`;
}

export function vaultKey() {
  return app.vaultKeys[app.vault] ?? null;
}

export function rotateKey(scope) {
  if (scope === 'vault' && !isVaultOwner()) return;
  const fresh = { secret: mockKey(scope === 'vault' ? 'ak_vault_' : 'ak_live_'), date: 'Rotated just now' };
  if (scope === 'vault') app.vaultKeys[app.vault] = fresh;
  else app.apiKey = fresh;
}

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

export function switchVault(id) {
  if (id === app.vault) return;
  app.vault = id;
  const head = route.parts[0];
  if (['chat', 'workflows'].includes(head) && route.parts[1]) go(`/${head === 'chat' ? '' : head}`);
}

export function createVault(name, look) {
  const id = nid('w');
  app.vaults.push(blankVault(id, name, look));
  app.vaultKeys[id] = { secret: mockKey('ak_vault_'), date: 'Created today' };
  app.vault = id;
  go('/');
}

export function deleteVault(id) {
  if (app.vaults.length < 2) return;
  app.vaults.splice(app.vaults.findIndex((w) => w.id === id), 1);
  app.vault = app.vaults[0].id;
  go('/');
}

export function newChat(title, { attachments = [], text } = {}) {
  const id = nid('n');
  const w = ws();
  const chat = { id, title: title || 'New chat', updated: 'Just now' };
  w.chats.unshift(chat);
  w.conversations[id] = [];
  if (text !== undefined) app.pending = { id, text, attachments };
  return id;
}

export function chatById(id) {
  return ws().chats.find((c) => c.id === id);
}

export function renameChat(id, title) {
  const chat = chatById(id);
  if (chat && title) chat.title = title;
}

export function togglePin(id) {
  const chat = chatById(id);
  if (!chat) return;
  chat.pinned = !chat.pinned;
}

export function pinnedFirst(chats) {
  return [...chats.filter((c) => c.pinned), ...chats.filter((c) => !c.pinned)];
}

export function removeChat(id) {
  const w = ws();
  const open = route.path === `/chat/${id}`;
  removeAt(w.chats, (c) => c.id === id, () => open && go('/'));
}


const kinds = [
  { kind: 'image', ext: 'JPG', size: '2.3 MB', base: 'scan' },
  { kind: 'pdf', ext: 'PDF', size: '640 KB', base: 'document' },
  { kind: 'recording', ext: 'M4A', size: '18 MB', base: 'recording', duration: '12:04' },
  { kind: 'document', ext: 'DOC', size: '16 KB', base: 'notes' }
];

export function uploadFile(folder) {
  const w = ws();
  const k = kinds[w.library.length % kinds.length];
  const n = String(w.library.length + 1).padStart(3, '0');
  const file = {
    id: nid('f'),
    name: `${k.base}-${n}.${k.ext.toLowerCase()}`,
    kind: k.kind,
    ext: k.ext,
    size: k.size,
    duration: k.duration,
    modified: 'Just now',
    owner: app.profile.name,
    folder: folder || w.folders[0] || 'Uploads',
    tone: (w.library.length % 4) + 1
  };
  if (!w.folders.includes(file.folder)) w.folders.push(file.folder);
  w.library.unshift(file);
  completeStep('upload');
  return file;
}

export function addFolder(name) {
  const w = ws();
  if (!name || w.folders.includes(name)) return;
  w.folders.push(name);
}

export function renameFile(id, name) {
  const f = ws().library.find((x) => x.id === id);
  if (f && name) f.name = name;
}

export function removeFile(id) {
  removeAt(ws().library, (f) => f.id === id);
}

export function downloadFile(file) {
  const blob = new Blob([`${file.name}\n${file.folder}, ${file.size}\n`], { type: 'text/plain' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `${file.name}.txt`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

export function invite({ emails, role }) {
  const w = ws();
  const added = [];
  for (const email of emails) {
    if (w.members.some((m) => m.email === email)) continue;
    const local = email.split('@')[0].replace(/[._-]+/g, ' ');
    const name = local.replace(/\b\w/g, (c) => c.toUpperCase());
    const m = { id: nid('u'), name, email, role, active: '', status: 'invited', tone: (w.members.length % 6) + 1 };
    w.members.push(m);
    added.push(m);
  }
  if (added.length) completeStep('invite');
  return added;
}

export function removeMember(id) {
  removeAt(ws().members, (x) => x.id === id);
}

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
    if (m.status === 'active' && m.name !== app.profile.name) out.set(m.email, { id: m.id, user: m.user, name: m.name, email: m.email, tone: m.tone });
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

export function workflowById(id) {
  return ws().workflows.find((w) => w.id === id);
}

function freshRun(wf) {
  const number = (wf.runs[0]?.number ?? 0) + 1;
  return {
    id: nid('r'),
    number,
    status: 'running',
    started: 'Just now',
    elapsed: '0s',
    seconds: 0,
    steps: wf.steps.map((label, i) => ({ id: `s${i}`, label, detail: '', status: i === 0 ? 'running' : 'pending', time: '' })),
    log: []
  };
}

export function runWorkflow(id) {
  const wf = workflowById(id);
  if (!wf) return;
  wf.runs.unshift(freshRun(wf));
  progress(wf.runs[0]);
  return wf.runs[0].id;
}

function progress(run) {
  setTimeout(() => {
    if (run.status !== 'running') return;
    if (!run.paused) {
      run.seconds = (run.seconds ?? 0) + 3;
      run.elapsed = `${run.seconds}s`;
      const s = run.steps.find((x) => x.status === 'running');
      if (s) {
        s.status = 'done';
        s.time = '3s';
      }
      const next = run.steps.find((x) => x.status === 'pending');
      if (next) next.status = 'running';
      else run.status = 'succeeded';
    }
    progress(run);
  }, 3000);
}

export function createWorkflow({ name, trigger, steps, agent = 'Assistant' }) {
  const id = nid('wf');
  ws().workflows.unshift({ id, name, agent, trigger, enabled: true, steps, runs: [] });
  completeStep('workflow');
  return id;
}

export function toggleWorkflow(id) {
  const wf = workflowById(id);
  if (!wf) return;
  wf.enabled = !wf.enabled;
}

export function renameWorkflow(id, name) {
  const wf = workflowById(id);
  if (wf && name) wf.name = name;
}

export function removeWorkflow(id) {
  removeAt(ws().workflows, (w) => w.id === id, () => go('/workflows'));
}

export function pauseRun(run) {
  run.paused = !run.paused;
}

export function stopRun(run) {
  run.status = 'stopped';
  run.paused = false;
  for (const s of run.steps) if (s.status === 'running' || s.status === 'waiting') s.status = 'failed';
}

export function resolveRun(run) {
  const s = run.steps.find((x) => x.status === 'waiting');
  if (s) {
    s.status = 'done';
    s.detail = 'Answered by you';
  }
  const next = run.steps.find((x) => x.status === 'pending');
  if (next) next.status = 'running';
  run.status = next ? 'running' : 'succeeded';
  if (next) progress(run);
}

export function steerRun(run, text) {
  run.log.push({ id: nid('l'), role: 'user', text });
  run.log.push({ id: nid('l'), role: 'assistant', blocks: [{ type: 'p', text: 'Got it. I’ll take that into account from the next step on.' }] });
}

export function connect(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  c.connected = true;
  c.account = app.profile.name.split(' ')[0].toLowerCase() + '@' + (c.category === 'Email' ? 'mail.com' : 'example.com');
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

function format(sec) {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return m ? `${m}m ${String(s).padStart(2, '0')}s` : `${s}s`;
}

function tick() {
  for (const w of app.vaults) {
    for (const t of w.workflows.flatMap((f) => f.runs)) {
      if (t.status !== 'running' || t.paused) continue;
      t.seconds = (t.seconds ?? 0) + 2;
      t.elapsed = format(t.seconds);
      const step = t.steps.find((s) => s.status === 'running');
      if (!step) continue;
      step.ticks = (step.ticks ?? 0) + 1;
      if (step.ticks >= 3) {
        step.status = 'done';
        step.time = format(step.ticks * 2);
        const next = t.steps.find((s) => s.status === 'pending');
        if (next) next.status = 'running';
        else t.status = 'succeeded';
      }
    }
  }
}

if (typeof window !== 'undefined') setInterval(tick, 2000);

export { initials };
