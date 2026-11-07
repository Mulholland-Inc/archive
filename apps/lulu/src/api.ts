import { useCallback, useEffect, useState } from 'react';

export type Row = Record<string, any>;

let bearer = '';
let workspace = '';
export const setBearer = (token: string) => { bearer = token; };
export const setWorkspace = (id: string) => { workspace = id; };

const headers = (extra: Record<string, string> = {}) => ({
  ...extra,
  ...(bearer ? { Authorization: `Bearer ${bearer}` } : {}),
  ...(workspace ? { 'Archive-Workspace': workspace } : {})
});

async function refusal(res: Response): Promise<string> {
  const text = (await res.text()).trim();
  try {
    const said = JSON.parse(text);
    return said.errors?.[0] ? `${said.errors[0].path}: ${said.errors[0].message}` : String(said.error ?? said.message ?? text);
  } catch { return text.slice(0, 200) || `${res.status}`; }
}

export class Status extends Error {
  constructor(public status: number, message: string) { super(message); }
}

async function send(path: string, init: RequestInit = {}, extra: Record<string, string> = {}): Promise<Response> {
  const res = await fetch(path, { ...init, headers: headers(extra) });
  if (!res.ok) throw new Status(res.status, await refusal(res));
  return res;
}
const json = (path: string, method: string, body?: unknown) =>
  send(path, { method, body: body === undefined ? undefined : JSON.stringify(body) }, body === undefined ? {} : { 'content-type': 'application/json' });

export const boot = <T = Row>(only?: string) => send(`/api/boot${only ? `?only=${only}` : ''}`).then(r => r.json() as Promise<T>);
export const get = <T = Row>(table: string, id: string) => send(`/api/${table}/${id}`).then(r => r.json() as Promise<T>);
export const create = <T = Row>(table: string, values: Row) => json(`/api/${table}`, 'POST', values).then(r => r.json() as Promise<T>);
export const patch = <T = Row>(table: string, id: string, values: Row) => json(`/api/${table}/${id}`, 'PATCH', values).then(r => r.json() as Promise<T>);
export const remove = (table: string, id: string) => json(`/api/${table}/${id}`, 'DELETE').then(() => undefined);
export const patchWorkspace = (values: Row) => json('/auth/workspace', 'PATCH', values).then(r => r.json());
export const methods = () => send('/auth/methods').then(r => r.json() as Promise<{ code: boolean; providers: { id: string; name?: string }[] }>);
export const verifyCode = (code: string) => json('/auth/totp/verify', 'POST', { code });
export const signOut = () => json('/auth/signout', 'POST').then(() => location.assign('/'));

export async function list<T = Row>(table: string, where: Record<string, string> = {}): Promise<T[]> {
  const rows: Row[] = [];
  for (let after = ''; ;) {
    const q = new URLSearchParams({ ...where, limit: '1000', ...(after ? { after } : {}) });
    const page = await send(`/api/${table}?${q}`).then(r => r.json() as Promise<Row[]>);
    rows.push(...page);
    if (page.length < 1000) return rows as T[];
    after = String(page[page.length - 1].id);
  }
}

export async function run<T = Row>(name: string, input: unknown = {}): Promise<T> {
  const out = await json(`/api/run/${name}`, 'POST', input).then(r => r.json() as Promise<{ result: T; error?: string }>);
  if (out.error) throw new Error(out.error.split('\n  at:')[0]);
  return out.result;
}

export type Stored = { id: string; name: string };
export function upload(file: File): Promise<Stored> {
  const form = new FormData();
  form.append('file', file);
  return send('/api/file', { method: 'POST', body: form }).then(r => r.json());
}
export async function uploadMany(files: File[], progress: (sent: number, total: number) => void) {
  const stored: Stored[] = [];
  const refused: { name: string; error: string }[] = [];
  let next = 0, done = 0;
  const lane = async () => {
    for (let i = next++; i < files.length; i = next++) {
      try { stored.push(await upload(files[i])); } catch (e) { refused.push({ name: files[i].name, error: (e as Error).message }); }
      progress(++done, files.length);
    }
  };
  await Promise.all([lane(), lane(), lane()]);
  return { stored, refused };
}

export const fileBlob = (id: string, part: string) => send(`/api/file/${id}/${part}`).then(r => r.blob());
const urls = new Map<string, Promise<string>>();
const waiting: (() => void)[] = [];
let flying = 0;
const slot = () => new Promise<void>(go => { if (flying < 8) { flying++; go(); } else waiting.push(go); });
const free = () => { const go = waiting.shift(); if (go) go(); else flying--; };
export function fileUrl(id: string, part = 'thumbnail'): Promise<string> {
  const key = `${id}/${part}`;
  let url = urls.get(key);
  if (!url) {
    url = slot().then(() => fileBlob(id, part).finally(free)).then(b => URL.createObjectURL(b));
    url.catch(() => urls.delete(key));
    urls.set(key, url);
  }
  return url;
}
export async function saveFile(id: string, name: string, part = 'content') {
  const a = document.createElement('a');
  a.href = URL.createObjectURL(await fileBlob(id, part));
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

export async function* turn(body: Row, signal: AbortSignal): AsyncGenerator<Row> {
  const res = await send('/api/turn', { method: 'POST', body: JSON.stringify(body), signal }, { 'content-type': 'application/json' });
  const reader = res.body!.pipeThrough(new TextDecoderStream()).getReader();
  let buf = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) return;
    buf += value;
    const lines = buf.split('\n');
    buf = lines.pop()!;
    for (const line of lines) if (line) yield JSON.parse(line);
  }
}
export const turns = (agent: string, limit: number) => send(`/api/turn?agent=${agent}&limit=${limit}&desc=1`).then(r => r.json() as Promise<Row[]>);
export const clearTurns = (agent: string) => json('/api/turn', 'POST', { agent, clear: true }).then(() => undefined);

type Entry = { data?: unknown; has: boolean; error?: string; stale: boolean; flight?: Promise<void>; again?: boolean; read: () => Promise<unknown>; tables: string[]; hearers: Set<() => void> };
const kept = new Map<string, Entry>();

function fetchEntry(e: Entry): Promise<void> {
  if (e.flight) { e.again = true; return e.flight; }
  e.stale = false;
  e.flight = e.read().then(
    data => { e.data = data; e.has = true; e.error = undefined; },
    err => { e.error = (err as Error).message; }
  ).then(() => {
    e.flight = undefined;
    e.hearers.forEach(h => h());
    if (e.again) { e.again = false; return fetchEntry(e); }
  });
  return e.flight;
}

function touch(e: Entry) {
  e.stale = true;
  if (e.hearers.size) void fetchEntry(e);
}

export const moved = () => kept.forEach(touch);

export function useRead<T>(key: string, read: () => Promise<T>, tables: string[]) {
  const [, redraw] = useState(0);
  let e = kept.get(key);
  if (!e) {
    e = { has: false, stale: true, read, tables, hearers: new Set() };
    kept.set(key, e);
    for (const old of kept.keys()) { if (kept.size <= 100) break; if (!kept.get(old)!.hearers.size) kept.delete(old); else break; }
  }
  e.read = read;
  const entry = e;
  useEffect(() => {
    const hear = () => redraw(n => n + 1);
    entry.hearers.add(hear);
    if (entry.stale) void fetchEntry(entry);
    return () => { entry.hearers.delete(hear); };
  }, [entry]);
  const reload = useCallback(() => fetchEntry(entry), [entry]);
  return { data: entry.has ? entry.data as T : undefined, error: entry.error, reload };
}

let socket = false;
export function listen() {
  if (socket || bearer) return;
  socket = true;
  const pending = new Set<string>();
  let soon = 0, last = 0;
  const fire = () => {
    soon = 0;
    last = Date.now();
    kept.forEach(e => { if (e.tables.some(t => pending.has(t))) touch(e); });
    pending.clear();
  };
  const dial = async (wait: number) => {
    const again = () => setTimeout(() => void dial(Math.min(wait * 2, 30000)), wait);
    let ticket: string;
    try { ticket = (await boot<{ live: string }>('live')).live; } catch { again(); return; }
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/live?ticket=${encodeURIComponent(ticket)}`);
    ws.onopen = () => { wait = 1000; };
    ws.onclose = again;
    ws.onmessage = m => {
      const frame = JSON.parse(m.data);
      if (frame.event !== 'change') return;
      for (const c of frame.changes ?? []) pending.add(String(c.table).replace(/^public\./, ''));
      if (!soon) soon = window.setTimeout(fire, Math.max(150, last + 2000 - Date.now()));
    };
  };
  void dial(1000);
}

function writeQuery(key: string, v: string, fallback: string) {
  const u = new URL(location.href);
  if (v && v !== fallback) u.searchParams.set(key, v); else u.searchParams.delete(key);
  if (u.href !== location.href) history.replaceState(history.state, '', u);
}
export function useQuery(key: string, fallback = ''): [string, (v: string) => void] {
  const [value, setValue] = useState(() => new URLSearchParams(location.search).get(key) ?? fallback);
  return [value || fallback, v => { setValue(v); writeQuery(key, v, fallback); }];
}

