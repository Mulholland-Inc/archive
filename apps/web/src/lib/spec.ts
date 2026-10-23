import type { ArchiveRecord, Field, Ontology, TypeDef } from './types';
import { iconNames, typeIconNames } from './components/Icon.svelte';

export type Kind =
  | 'text'
  | 'richtext'
  | 'integer'
  | 'number'
  | 'decimal'
  | 'boolean'
  | 'enum'
  | 'reference'
  | 'date'
  | 'timestamp'
  | 'money'
  | 'email'
  | 'phone'
  | 'url'
  | 'file'
  | 'object'
  | 'string';

const direct: { [name: string]: Kind } = {
  text: 'text',
  richtext: 'richtext',
  integer: 'integer',
  number: 'number',
  decimal: 'decimal',
  boolean: 'boolean',
  enum: 'enum',
  reference: 'reference',
  date: 'date',
  timestamp: 'timestamp',
  money: 'money',
  email: 'email',
  phone: 'phone',
  url: 'url',
  file: 'file'
};

export function kindOf(o: Ontology | null, type: string): Kind {
  const types = o?.spec.types ?? {};
  for (let name: string | undefined = type, i = 0; name && i < 20; i++) {
    if (direct[name]) return direct[name];
    const t: TypeDef | undefined = types[name];
    if (!t) break;
    if (!t.extends) {
      const schemaType = (t.schema as { type?: string } | undefined)?.type;
      if (t.fields && Object.keys(t.fields).length) return 'object';
      if (schemaType === 'object') return 'object';
      if (schemaType === 'boolean') return 'boolean';
      if (schemaType === 'integer') return 'integer';
      if (schemaType === 'number') return 'number';
      return 'string';
    }
    name = t.extends;
  }
  return 'string';
}


export function objectTypes(o: Ontology | null): string[] {
  if (!o) return [];
  return Object.keys(o.spec.types)
    .filter((n) => !o.builtin.includes(n))
    .sort((a, b) => a.localeCompare(b));
}

export function typeIcon(o: Ontology | null, type: string): string {
  const types = objectTypes(o);
  const own = (t: string) => {
    const i = o?.spec.types[t]?.icon;
    return i && iconNames.includes(i) ? i : '';
  };
  if (own(type)) return own(type);
  const used = new Set(types.map(own).filter(Boolean));
  for (const t of [...types.filter((t) => t !== type && !own(t)), type]) {
    let h = 0;
    for (const c of t) h = (h * 31 + c.charCodeAt(0)) >>> 0;
    const free = typeIconNames.filter((i) => !used.has(i));
    const icon = free.length ? free[h % free.length] : 'table';
    if (t === type) return icon;
    used.add(icon);
  }
  return 'table';
}

export function fieldTypes(o: Ontology | null): string[] {
  if (!o) return [];
  return [...o.builtin].sort((a, b) => a.localeCompare(b));
}

export function fieldsOf(o: Ontology | null, type: string): [string, Field][] {
  const types = o?.spec.types ?? {};
  const chain: TypeDef[] = [];
  for (let name: string | undefined = type, i = 0; name && types[name] && i < 20; i++) {
    chain.unshift(types[name]);
    name = types[name].extends;
  }
  const merged: { [k: string]: Field } = {};
  for (const t of chain) Object.assign(merged, t.fields ?? {});
  return Object.entries(merged).sort(([a, fa], [b, fb]) => Number(!!fb.required) - Number(!!fa.required) || a.localeCompare(b));
}

const titleFields = ['name', 'title', 'subject', 'number', 'label'];

export function titleField(o: Ontology | null, type: string): string | undefined {
  const fields = fieldsOf(o, type);
  for (const f of titleFields) if (fields.some(([n]) => n === f)) return f;
  return fields.find(([, f]) => kindOf(o, f.type) === 'text' && !f.list)?.[0];
}

export function recordTitle(o: Ontology | null, r: ArchiveRecord): string {
  const f = titleField(o, r.type);
  const v = f ? r.data[f] : undefined;
  if (typeof v === 'string' && v.trim()) return v;
  if (typeof v === 'number') return String(v);
  return `${humanize(r.type)} ${r.id.slice(-6)}`;
}

const acronyms = new Set(['iban', 'bic', 'url', 'vat', 'id', 'ip', 'uuid', 'gtin', 'isbn', 'isin', 'lei', 'mime', 'us', 'nl', 'ein', 'ssn', 'bsn', 'api', 'pdf', 'imap', 'mcp', 'eu', 'uk']);

export function humanize(name: string) {
  const words = name.replace(/[_-]+/g, ' ').trim().split(/\s+/);
  const out = words.map((w, i) => (acronyms.has(w.toLowerCase()) ? w.toUpperCase() : i === 0 ? w.charAt(0).toUpperCase() + w.slice(1) : w));
  return out.join(' ');
}

export function isSecret(v: unknown): boolean {
  return !!v && typeof v === 'object' && '$secret' in (v as object);
}

const dateFmt = new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'short', day: 'numeric', timeZone: 'UTC' });
const timeFmt = new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });

export function formatDate(iso: string) {
  const d = new Date(iso.length === 10 ? `${iso}T00:00:00Z` : iso);
  return Number.isNaN(d.getTime()) ? iso : dateFmt.format(d);
}

export function formatTime(iso: string) {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : timeFmt.format(d);
}

export function relative(iso?: string) {
  if (!iso) return '';
  const d = new Date(iso).getTime();
  const s = Math.round((Date.now() - d) / 1000);
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
  const abs = Math.abs(s);
  if (abs < 45) return rtf.format(-s, 'second');
  if (abs < 2700) return rtf.format(-Math.round(s / 60), 'minute');
  if (abs < 64800) return rtf.format(-Math.round(s / 3600), 'hour');
  if (abs < 2592000) return rtf.format(-Math.round(s / 86400), 'day');
  return formatDate(iso);
}

export function formatBytes(n: number) {
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 1 : 0)} ${units[i]}`;
}

function formatOne(o: Ontology | null, f: Field, v: unknown): string {
  if (v === null || v === undefined || v === '') return '';
  if (isSecret(v)) return '••••••';
  if (f.type === 'percentage' && !isNaN(Number(v))) return new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 2 }).format(Number(v));
  const kind = kindOf(o, f.type);
  switch (kind) {
    case 'boolean':
      return v ? 'Yes' : 'No';
    case 'date':
      return typeof v === 'string' ? formatDate(v) : String(v);
    case 'timestamp': {
      const t = v as { utc?: string; zone?: string };
      return t.utc ? formatTime(t.utc) : JSON.stringify(v);
    }
    case 'money': {
      const m = v as { amount?: string; currency?: string };
      if (!m.amount || !m.currency) return JSON.stringify(v);
      try {
        return new Intl.NumberFormat(undefined, { style: 'currency', currency: m.currency }).format(Number(m.amount));
      } catch {
        return `${m.amount} ${m.currency}`;
      }
    }
    case 'integer':
    case 'number':
      return typeof v === 'number' ? new Intl.NumberFormat().format(v) : String(v);
    case 'file': {
      const file = v as { name?: string };
      return file.name ?? 'File';
    }
    case 'enum':
      return humanize(String(v));
    case 'object':
    case 'string':
      if (typeof v === 'object') return summarize(v);
      return String(v);
    default:
      return typeof v === 'object' ? summarize(v) : String(v);
  }
}

function summarize(v: unknown): string {
  if (Array.isArray(v)) return v.map(summarize).join(', ');
  if (v && typeof v === 'object') {
    const parts = Object.values(v as object).filter((x) => x !== null && x !== '' && typeof x !== 'object');
    if (parts.length) return parts.join(', ');
    return JSON.stringify(v);
  }
  return String(v ?? '');
}

export function formatValue(o: Ontology | null, f: Field, v: unknown): string {
  if (f.list && Array.isArray(v)) return v.map((x) => formatOne(o, f, x)).filter(Boolean).join(', ');
  return formatOne(o, f, v);
}

export function inputValue(o: Ontology | null, f: Field, v: unknown): string {
  if (v === null || v === undefined) return '';
  if (isSecret(v)) return '';
  const kind = kindOf(o, f.type);
  if (f.list && Array.isArray(v)) return v.map((x) => inputValue(o, { ...f, list: false }, x)).join('\n');
  if (kind === 'money') {
    const m = v as { amount?: string; currency?: string };
    return m.amount && m.currency ? `${m.amount} ${m.currency}` : '';
  }
  if (kind === 'timestamp') return (v as { utc?: string }).utc ?? '';
  if (typeof v === 'object') return summarize(v);
  return String(v);
}

export function parseInput(o: Ontology | null, f: Field, s: string | boolean): unknown {
  if (typeof s === 'boolean') return s;
  const kind = kindOf(o, f.type);
  if (f.list) {
    const items = s
      .split('\n')
      .map((x) => x.trim())
      .filter(Boolean);
    return items.length ? items.map((x) => parseInput(o, { ...f, list: false }, x)) : null;
  }
  const t = s.trim();
  if (t === '') return null;
  if (kind === 'integer') return /^-?\d+$/.test(t) ? Number(t) : t;
  if (kind === 'number') return Number.isFinite(Number(t)) ? Number(t) : t;
  return t;
}
