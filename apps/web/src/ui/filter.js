import { api, show } from '../api.js';

export const title = (c) => c.label ?? c.name.replaceAll('_', ' ');

export function kind(c, types = {}) {
  if (c.type === 'uuid' || c.ref) return null;
  if (c.type === 'bool') return 'bool';
  const t = types[`${c.schema}.${c.type}`];
  if (t?.enum?.length) return 'enum';
  const base = t?.base || c.type;
  if (/^(date|timestamp|timestamptz)$/.test(base) || c.type === 'timestamp') return 'date';
  if (/^(int[248]|numeric|float[48]|money|quantity)$/.test(base) || /^(money|quantity)$/.test(c.type)) return 'number';
  return 'text';
}

export function options(c, types = {}) {
  if (c.type === 'bool') return [['true', 'yes'], ['false', 'no']];
  return (types[`${c.schema}.${c.type}`]?.enum ?? []).map((v) => [v, v.replaceAll('_', ' ')]);
}

function scalar(v) {
  if (v && typeof v === 'object' && !Array.isArray(v)) return v.utc ?? v.amount ?? v.value ?? show(v);
  return v;
}

function test(want, k) {
  if (k === 'enum' || k === 'bool') {
    const set = new Set(want.split(','));
    return (v) => (Array.isArray(v) ? v : [v]).some((x) => set.has(String(x)));
  }
  if (k === 'date') {
    const [from = '', to = ''] = want.split('..');
    return (v) => {
      if (v === null || v === undefined || v === '') return false;
      const d = String(scalar(v)).slice(0, 10);
      return (!from || d >= from) && (!to || d <= to);
    };
  }
  const w = want.toLowerCase();
  return (v) => show(v).toLowerCase().includes(w);
}

function key(v, k) {
  if (v === null || v === undefined || v === '') return null;
  const s = scalar(v);
  if (k === 'number') return Number(s);
  if (k === 'date') return String(s);
  return show(v);
}

export function order(rows, sort, columns, types = {}) {
  if (!sort) return rows;
  const c = columns.find((x) => x.name === sort.col);
  if (!c) return rows;
  const k = kind(c, types);
  const dir = sort.desc ? -1 : 1;
  return [...rows].sort((a, b) => {
    const x = key(a[c.name], k);
    const y = key(b[c.name], k);
    if (x === null || y === null) return x === y ? 0 : x === null ? 1 : -1;
    return dir * (typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), undefined, { numeric: true, sensitivity: 'base' }));
  });
}

export function keep(rows, values, columns, types = {}) {
  const tests = Object.entries(values).filter(([, v]) => v && v !== '..').flatMap(([name, v]) => {
    const c = columns.find((x) => x.name === name);
    const k = c && kind(c, types);
    if (!k) return [];
    const t = test(v, k);
    return [(r) => t(r[name])];
  });
  return tests.length ? rows.filter((r) => tests.every((t) => t(r))) : rows;
}

export function filtering(q) {
  return Object.values(q.values).some((v) => v && v !== '..');
}

export function read() {
  const p = new URLSearchParams(location.search);
  const s = p.get('sort');
  const values = {};
  for (const [k, v] of p) if (k !== 'q' && k !== 'sort' && v) values[k] = v;
  return { query: p.get('q') ?? '', values, sort: s ? { col: s.replace(/^-/, ''), desc: s.startsWith('-') } : null };
}

export function write(q) {
  const enc = (s) => encodeURIComponent(s).replace(/%3A/gi, ':').replace(/%2C/gi, ',');
  const p = [];
  if (q.query) p.push(`q=${enc(q.query)}`);
  for (const [k, v] of Object.entries(q.values)) if (v && v !== '..') p.push(`${enc(k)}=${enc(v)}`);
  if (q.sort) p.push(`sort=${q.sort.desc ? '-' : ''}${enc(q.sort.col)}`);
  const search = p.length ? `?${p.join('&')}` : '';
  if (search !== location.search) history.replaceState(history.state, '', location.pathname + search + location.hash);
}

export function cycle(sort, col) {
  if (sort?.col !== col) return { col, desc: false };
  return sort.desc ? null : { col, desc: true };
}

export async function fetchRows(name, { query = '', all = false, after = '' } = {}) {
  if (query.trim()) {
    const hits = await api(`/api/search?${new URLSearchParams({ q: query.trim(), type: name, limit: '100' })}`);
    return { rows: hits.map((h) => h.data), more: false };
  }
  const size = all ? 1000 : 100;
  const out = [];
  for (;;) {
    const q = new URLSearchParams({ limit: String(size) });
    if (after) q.set('after', after);
    const next = await api(`/api/${name}?${q}`);
    out.push(...next);
    if (next.length < size) return { rows: out, more: false };
    after = next.at(-1).id;
    if (!all) return { rows: out, more: true };
  }
}
