import { me, initials } from './data.js';
import { seed, blankVault } from './seed.js';
import { toast } from './toast.svelte.js';
import { go, route } from './router.svelte.js';

let counter = 0;
const nid = (prefix) => `${prefix}${Date.now().toString(36)}${(counter++).toString(36)}`;

export const app = $state({
  vaults: seed(),
  vault: 'okafor',
  collapsed: typeof localStorage !== 'undefined' && localStorage.getItem('archive.sidebar') === 'collapsed',
  drawer: false,
  palette: false,
  help: false,
  incognito: false,
  model: 'pro',
  pending: null,
  signedOut: false,
  profile: { name: me.name, first: me.first, work: 'archivist', prefs: 'Keep answers short. Use British spelling. When you quote a letter, keep the original spelling and punctuation.', tone: me.tone, notify: { done: true, mentions: true, digest: true, invites: false } }
});

export function ws() {
  return app.vaults.find((w) => w.id === app.vault) ?? app.vaults[0];
}

export const currentVault = ws;

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

function removeAt(list, predicate, label, after) {
  const i = list.findIndex(predicate);
  if (i < 0) return null;
  const [item] = list.splice(i, 1);
  after?.(item);
  toast(label, { action: { label: 'Undo', run: () => list.splice(Math.min(i, list.length), 0, item) } });
  return item;
}

export function switchVault(id) {
  if (id === app.vault) return;
  app.vault = id;
  const head = route.parts[0];
  if (['chat', 'projects', 'workflows'].includes(head) && route.parts[1]) go(`/${head === 'chat' ? '' : head}`);
  toast(`Switched to ${ws().name}`);
}

export function createVault(name, look) {
  const id = nid('w');
  app.vaults.push(blankVault(id, name, look));
  app.vault = id;
  go('/');
  toast(`Created ${name}`, { tone: 'success' });
}

export function deleteVault(id) {
  if (app.vaults.length < 2) {
    toast('You need at least one vault', { tone: 'danger' });
    return;
  }
  const i = app.vaults.findIndex((w) => w.id === id);
  const [gone] = app.vaults.splice(i, 1);
  app.vault = app.vaults[0].id;
  go('/');
  toast(`${gone.name} deleted`, {
    action: {
      label: 'Undo',
      run: () => {
        app.vaults.splice(i, 0, gone);
        app.vault = gone.id;
      }
    }
  });
}

export function newChat(title, { project, attachments = [], text } = {}) {
  const id = nid('n');
  const w = ws();
  const chat = { id, title: title || 'New chat', group: 'Today', updated: 'Just now', project, hidden: app.incognito };
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

export function toggleStar(id) {
  const chat = chatById(id);
  if (!chat) return;
  chat.starred = !chat.starred;
  toast(chat.starred ? 'Starred' : 'Removed from starred');
}

export function removeChat(id) {
  const w = ws();
  const open = route.path === `/chat/${id}`;
  removeAt(w.chats, (c) => c.id === id, 'Chat deleted', () => open && go('/'));
}

export function moveChat(id, projectId) {
  const chat = chatById(id);
  if (!chat) return;
  chat.project = chat.project === projectId ? undefined : projectId;
  const p = ws().projects.find((x) => x.id === projectId);
  toast(chat.project ? `Moved to ${p?.name}` : `Removed from ${p?.name}`);
}

export async function copyLink(path) {
  const url = `${location.origin}${location.pathname}#${path}`;
  try {
    await navigator.clipboard.writeText(url);
  } catch {}
  toast('Link copied');
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
  toast(`Uploaded ${file.name}`, { tone: 'success' });
  return file;
}

export function addFolder(name) {
  const w = ws();
  if (!name || w.folders.includes(name)) return;
  w.folders.push(name);
  toast(`Folder “${name}” created`, { tone: 'success' });
}

export function renameFile(id, name) {
  const f = ws().library.find((x) => x.id === id);
  if (f && name) f.name = name;
}

export function removeFile(id) {
  removeAt(ws().library, (f) => f.id === id, 'Moved to trash');
}

export function downloadFile(file) {
  const blob = new Blob([`${file.name}\n${file.folder} · ${file.size}\n`], { type: 'text/plain' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `${file.name}.txt`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

export function createProject({ name, instructions }) {
  const id = nid('p');
  ws().projects.unshift({ id, name, instructions: instructions || '', updated: 'Just now', files: 0, chats: 0, members: [app.profile.name], fileIds: [] });
  toast(`Created ${name}`, { tone: 'success' });
  return id;
}

export function projectById(id) {
  return ws().projects.find((p) => p.id === id);
}

export function removeProject(id, label = 'Project deleted') {
  removeAt(ws().projects, (p) => p.id === id, label, () => go('/projects'));
}

export function addProjectFile(id) {
  const p = projectById(id);
  if (!p) return;
  const file = uploadFile(p.name);
  p.fileIds.unshift(file.id);
}

export function invite({ emails, role, groups = [], project }) {
  const w = ws();
  const added = [];
  for (const email of emails) {
    if (w.members.some((m) => m.email === email)) continue;
    const local = email.split('@')[0].replace(/[._-]+/g, ' ');
    const name = local.replace(/\b\w/g, (c) => c.toUpperCase());
    const m = { id: nid('u'), name, email, role, groups: [...groups], active: '', status: 'invited', tone: (w.members.length % 6) + 1 };
    w.members.push(m);
    added.push(m);
  }
  if (project) {
    const p = projectById(project);
    for (const m of added) if (p && !p.members.includes(m.name)) p.members.push(m.name);
  }
  toast(added.length === 1 ? `Invited ${added[0].email}` : `${added.length} people invited`, { tone: 'success' });
  return added;
}

export function removeMember(id) {
  const m = ws().members.find((x) => x.id === id);
  removeAt(ws().members, (x) => x.id === id, m?.status === 'invited' ? 'Invitation revoked' : `${m?.name} removed`);
}

export function createGroup(name) {
  const w = ws();
  if (!name || w.groups.some((g) => g.name === name)) return;
  w.groups.push({ id: nid('g'), name });
  toast(`Group “${name}” created`, { tone: 'success' });
}

export function setGroupMembers(groupName, ids) {
  for (const m of ws().members) {
    const has = m.groups.includes(groupName);
    if (ids.includes(m.id) && !has) m.groups.push(groupName);
    if (!ids.includes(m.id) && has) m.groups = m.groups.filter((g) => g !== groupName);
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
  toast(`Run #${wf.runs[0].number} started`);
  return wf.runs[0].id;
}

export function createWorkflow({ name, trigger, steps, agent = 'Assistant' }) {
  const id = nid('wf');
  ws().workflows.unshift({ id, name, agent, trigger, enabled: true, steps, runs: [] });
  toast(`Created ${name}`, { tone: 'success' });
  return id;
}

export function toggleWorkflow(id) {
  const wf = workflowById(id);
  if (!wf) return;
  wf.enabled = !wf.enabled;
  toast(wf.enabled ? `${wf.name} turned on` : `${wf.name} turned off`);
}

export function renameWorkflow(id, name) {
  const wf = workflowById(id);
  if (wf && name) wf.name = name;
}

export function removeWorkflow(id) {
  removeAt(ws().workflows, (w) => w.id === id, 'Workflow deleted', () => go('/workflows'));
}

export function pauseRun(run) {
  run.paused = !run.paused;
  toast(run.paused ? 'Run paused' : 'Run resumed');
}

export function stopRun(run) {
  run.status = 'stopped';
  run.paused = false;
  for (const s of run.steps) if (s.status === 'running' || s.status === 'waiting') s.status = 'failed';
  toast('Run stopped');
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
  toast(`${c.name} connected`, { tone: 'success' });
}

export function disconnect(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  const before = { ...c };
  c.connected = false;
  c.account = '';
  c.lastSync = '';
  toast(`${c.name} disconnected`, { action: { label: 'Undo', run: () => Object.assign(c, before) } });
}

export function syncConnector(id) {
  const c = ws().connectors.find((x) => x.id === id);
  if (!c) return;
  c.lastSync = 'Just now';
  c.items += 3;
  toast(`${c.name} synced`);
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
