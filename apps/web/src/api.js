export class Unauthorized extends Error {}

export function go(path) {
  history.pushState(null, '', path);
  dispatchEvent(new PopStateEvent('popstate'));
}

export function space() {
  try {
    return localStorage.space ?? '';
  } catch {
    return '';
  }
}

export function setSpace(name) {
  try {
    if (name) localStorage.space = name;
    else delete localStorage.space;
  } catch {}
}

async function send(path, { method = 'GET', body, raw, signal } = {}) {
  const init = { method, headers: {}, signal };
  if (space()) init.headers['Archive-Space'] = space();
  if (raw !== undefined) init.body = raw;
  else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers['Content-Type'] = 'application/json';
  }
  const r = await fetch(path, init);
  if (r.status === 401) throw new Unauthorized();
  if (!r.ok && r.status !== 204) {
    const out = await r.json().catch(() => ({}));
    throw new Error(out.error ?? r.statusText);
  }
  return r;
}

export async function api(path, opts) {
  const r = await send(path, opts);
  return r.status === 204 ? null : r.json();
}

export async function* chat(body, signal) {
  const r = await send('/api/chat', { method: 'POST', body, signal });
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
