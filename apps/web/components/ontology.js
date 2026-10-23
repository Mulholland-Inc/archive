import { api, spaceQuery } from './api.js';
import { endpoints } from './endpoints.js';

let specPromise;

export function loadSpec(fresh = false) {
  if (!specPromise || fresh) specPromise = api(endpoints.types).then((d) => ({ ...d.spec, builtin: new Set(d.builtin) }));
  return specPromise;
}

export const isUserType = (spec, name) => Boolean(spec.types[name]) && !spec.builtin.has(name);

export const userTypes = (spec) => Object.keys(spec.types).filter((n) => isUserType(spec, n)).sort();

export const label = (name) => (name ?? '').replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase());

const names = {
  iban: 'IBAN', bic: 'BIC', vat_id: 'VAT ID', url: 'URL', ip: 'IP address', gtin: 'GTIN', isbn: 'ISBN', lei: 'LEI',
  isin: 'ISIN', uuid: 'UUID', mime: 'MIME type', us_ssn: 'US SSN', us_ein: 'US EIN', us_routing: 'US routing number',
  nl_bsn: 'BSN', enum: 'Choice', richtext: 'Rich text',
};

export const typeLabel = (name) => names[name] ?? label(name);

export function chain(spec, name) {
  const out = [];
  for (let n = name; n && spec.types[n] && !out.includes(n); n = spec.types[n].extends) out.push(n);
  return out;
}

export function fieldsOf(spec, name) {
  const fields = {};
  for (const n of chain(spec, name).reverse()) {
    for (const [key, field] of Object.entries(spec.types[n].fields ?? {})) fields[key] = { key, field, from: n };
  }
  const list = Object.values(fields).sort((a, b) => a.key.localeCompare(b.key));
  const title = titleKey(list);
  return title ? [list.find((f) => f.key === title), ...list.filter((f) => f.key !== title)] : list;
}

const titleNames = ['name', 'title', 'number', 'subject', 'label'];

function titleKey(list) {
  const textual = list.filter((f) => ['text', 'name'].includes(f.field.type) && !f.field.list);
  return (
    titleNames.find((n) => textual.some((f) => f.key === n)) ??
    textual.find((f) => f.field.required)?.key ??
    textual[0]?.key
  );
}

export function recordTitle(spec, record) {
  const data = record?.data ?? {};
  const key = titleKey(fieldsOf(spec, record?.type));
  const v = key ? data[key] : undefined;
  if (v && typeof v === 'object') return v.full ?? Object.values(v).join(' ');
  return v ? String(v) : `Untitled ${record?.type ?? ''}`.trim();
}

const records = new Map();

export function loadRecord(type, id) {
  const key = `${type}/${id}`;
  if (!records.has(key)) records.set(key, api(endpoints.record(type, id)).catch(() => null));
  return records.get(key);
}

const read = (key, fallback) => {
  try {
    return JSON.parse(localStorage.getItem(key)) ?? fallback;
  } catch {
    return fallback;
  }
};

export const draft = {
  get: () => read('archive.draft', []),
  set: (ops) => localStorage.setItem('archive.draft', JSON.stringify(ops)),
  add(op) {
    this.set([...this.get(), op]);
  },
};

export function describeOp(op) {
  const def = op.def ?? {};
  const t = typeLabel(def.type);
  const kind =
    def.type === 'reference' ? `reference to ${label(def.to)}`
    : def.type === 'enum' ? `choice of ${(def.enum ?? []).join(', ')}`
    : /^[A-Z]{2}/.test(t) ? t : t.toLowerCase();
  switch (op.op) {
    case 'add_type':
      return def.extends ? `Add type ${label(op.type)}, based on ${label(def.extends)}` : `Add type ${label(op.type)}`;
    case 'remove_type':
      return `Remove type ${label(op.type)}`;
    case 'change_type':
      return `Change type ${label(op.type)}`;
    case 'add_field':
      return `Add field ${op.field} to ${label(op.type)}: ${kind}`;
    case 'change_field':
      return `Change field ${op.field} on ${label(op.type)} to ${kind}`;
    case 'rename_field':
      return `Rename ${op.field} on ${label(op.type)} to ${op.to}`;
    case 'remove_field':
      return `Remove ${op.field} from ${label(op.type)}`;
  }
  return `${op.op} ${op.type}`;
}

export async function loadAll(type, where) {
  const out = [];
  const space = await spaceQuery();
  for (let after; ; ) {
    const page = await api(endpoints.records(type), { query: { ...where, ...space, limit: 1000, after } });
    out.push(...page);
    if (page.length < 1000) return out;
    after = page.at(-1).id;
  }
}
