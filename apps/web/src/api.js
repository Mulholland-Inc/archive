export class Unauthorized extends Error {}

export function go(path) {
  history.pushState(null, '', path);
  dispatchEvent(new PopStateEvent('popstate'));
}

export async function api(path, { method = 'GET', body, raw } = {}) {
  const init = { method, headers: {} };
  if (raw !== undefined) init.body = raw;
  else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers['Content-Type'] = 'application/json';
  }
  const r = await fetch(path, init);
  if (r.status === 401) throw new Unauthorized();
  if (r.status === 204) return null;
  const out = await r.json();
  if (!r.ok) throw new Error(out.error ?? r.statusText);
  return out;
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
