export class Unauthorized extends Error {}

export function go(path) {
  history.pushState(null, '', path);
  dispatchEvent(new PopStateEvent('popstate'));
}

const cache = new Map();
const ttl = 15000;
let space = '';

export function scope(name) {
  space = name;
  cache.clear();
}

export function api(path, opts = {}) {
  if ((opts.method ?? 'GET') !== 'GET') {
    cache.clear();
    return request(path, opts);
  }
  const hit = cache.get(path);
  if (hit && Date.now() - hit.at < ttl) return hit.out;
  const out = request(path, opts);
  cache.set(path, { at: Date.now(), out });
  out.catch(() => cache.get(path)?.out === out && cache.delete(path));
  return out;
}

async function send(path, { method = 'GET', body, raw, signal } = {}) {
  const init = { method, headers: {}, signal };
  if (space) init.headers['Archive-Space'] = space;
  if (raw !== undefined) init.body = raw;
  else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers['Content-Type'] = 'application/json';
  }
  const r = await fetch(path, init);
  if (r.status === 401) throw new Unauthorized();
  if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? r.statusText);
  return r;
}

async function request(path, opts) {
  const r = await send(path, opts);
  return r.status === 204 ? null : r.json();
}

export async function* chat(messages, signal) {
  const r = await send('/api/chat', { method: 'POST', body: { messages }, signal });
  const reader = r.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += value;
    const lines = buf.split('\n');
    buf = lines.pop();
    for (const line of lines) if (line) yield JSON.parse(line);
  }
}

let ready;
export function editor() {
  ready ??= new Promise((resolve) => {
    window.pagesReady = () => resolve(window.pages);
    const go = new window.Go();
    WebAssembly.instantiateStreaming(fetch('/editor.wasm'), go.importObject).then((r) => go.run(r.instance));
  });
  return ready;
}

let catalog = null;
export function known(c) {
  catalog = c;
}

export function preload(path) {
  const [, first = '', second = ''] = path.split(/[?#]/)[0].split('/').map(decodeURIComponent);
  const get = (p) => api(p).catch(() => {});
  if (first === 'files' || (first === 'pages' && !second)) return get('/api/files?limit=500');
  if (first === 'pages') {
    get(`/api/files/${second}`);
    return editor();
  }
  const t = catalog?.tables[first === 'views' ? second : first];
  if (!t) return;
  get(`/api/${t.name}?limit=${t.kind === 'view' && t.layout !== 'table' && t.by ? 1000 : 100}`);
}

export async function run(code) {
  const out = await api('/api/run', { method: 'POST', body: { code } });
  if (out.error) throw new Error(out.error);
  return out.result;
}

export const hidden = new Set(['id', 'space', 'created_at', 'updated_at', 'search']);

export function show(v) {
  if (v === null || v === undefined) return '';
  if (Array.isArray(v)) return v.map(show).join(', ');
  if (typeof v === 'object') {
    if ('amount' in v && 'currency' in v) return `${v.amount} ${v.currency}`;
    if ('utc' in v) return new Date(v.utc).toLocaleString();
    if ('value' in v && 'unit' in v) return `${v.value} ${v.unit}`;
    if ('full' in v) return v.full ?? [v.given, v.family].filter(Boolean).join(' ');
    if ('lines' in v) return [...(v.lines ?? []), v.locality, v.country].filter(Boolean).join(', ');
    if ('lat' in v) return `${v.lat.toFixed(4)}, ${v.lon.toFixed(4)}`;
    return Object.values(v).filter((x) => x !== null && x !== '').map(show).join(' ');
  }
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(v)) return new Date(v).toLocaleString();
  return String(v);
}

export function size(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1 << 20) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / (1 << 20)).toFixed(1)} MB`;
}

export function choice(c, types) {
  return c.type === 'bool' || !!types[`${c.schema}.${c.type}`]?.enum?.length;
}

function key(v) {
  if (v === null || v === undefined || v === '') return null;
  if (typeof v === 'object' && !Array.isArray(v)) {
    if ('amount' in v) return Number(v.amount);
    if ('utc' in v) return v.utc;
    if ('value' in v) return Number(v.value);
  }
  return typeof v === 'number' ? v : typeof v === 'string' ? v : show(v);
}

export function arrange(rows, columns, types, { filters = {}, sort } = {}) {
  const tests = Object.entries(filters).filter(([, f]) => f !== '').map(([name, f]) => {
    const c = columns.find((x) => x.name === name);
    if (c && choice(c, types)) return (r) => String(r[name]) === f;
    const want = f.toLowerCase();
    return (r) => show(r[name]).toLowerCase().includes(want);
  });
  const out = rows.filter((r) => tests.every((t) => t(r)));
  if (!sort) return out;
  const dir = sort.desc ? -1 : 1;
  return out.sort((a, b) => {
    const x = key(a[sort.name]), y = key(b[sort.name]);
    if (x === null || y === null) return x === y ? 0 : x === null ? 1 : -1;
    return dir * (typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), undefined, { numeric: true }));
  });
}

export async function rows(name, columns, types, { query = '', filters = {}, all = false, after = '' } = {}) {
  if (query.trim()) {
    const hits = await api(`/api/search?${new URLSearchParams({ q: query, type: name, limit: '100' })}`);
    return { rows: hits.map((h) => h.data), more: false };
  }
  const where = Object.entries(filters).filter(([k, v]) => v !== '' && choice(columns.find((c) => c.name === k) ?? {}, types));
  const out = [];
  const size = all ? 1000 : 100;
  for (;;) {
    const q = new URLSearchParams([...where, ['limit', String(size)]]);
    if (after) q.set('after', after);
    const next = await api(`/api/${name}?${q}`);
    out.push(...next);
    if (next.length < size) return { rows: out, more: false };
    after = next.at(-1).id;
    if (!all) return { rows: out, more: true };
  }
}
