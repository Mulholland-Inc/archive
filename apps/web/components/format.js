import { chain } from './ontology.js';

export const locale = () => localStorage.getItem('archive.locale') || navigator.language;

export function setLocale(value) {
  if (value) localStorage.setItem('archive.locale', value);
  else localStorage.removeItem('archive.locale');
}

export function kind(spec, field) {
  if (['reference', 'enum'].includes(field.type)) return field.type;
  const names = chain(spec, field.type);
  return names.map((n) => spec.types[n].format).find(Boolean) ?? names.at(-1) ?? field.type;
}

export const toDate = (iso) => {
  const [y, m, d] = String(iso).slice(0, 10).split('-').map(Number);
  return new Date(y, m - 1, d);
};

export const isoDate = (date) =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;

const dateText = (iso) => new Intl.DateTimeFormat(locale(), { dateStyle: 'medium' }).format(toDate(iso));

const number = (v, options) => new Intl.NumberFormat(locale(), options).format(v);

const country = (code) => {
  try {
    return new Intl.DisplayNames(locale(), { type: 'region' }).of(code) ?? code;
  } catch {
    return code;
  }
};

const twoDigitCodes = new Set('20 27 30 31 32 33 34 36 39 40 41 43 44 45 46 47 48 49 51 52 53 54 55 56 57 58 60 61 62 63 64 65 66 81 82 84 86 90 91 92 93 94 95 98'.split(' '));

const phone = (v) => {
  const digits = String(v).replace(/^\+/, '');
  const cc = /^[17]/.test(digits) ? 1 : twoDigitCodes.has(digits.slice(0, 2)) ? 2 : 3;
  const groups = digits.slice(cc).match(/\d{1,3}/g) ?? [];
  if (groups.length > 1 && groups.at(-1).length === 1) groups.push(groups.splice(-2).join(''));
  return `+${digits.slice(0, cc)} ${groups.join(' ')}`;
};

const formats = {
  phone,
  date: dateText,
  timestamp: (v) =>
    new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short', timeZone: v.zone || 'UTC' }).format(new Date(v.utc)),
  time: (v) => {
    const [h, m] = v.time.split(':').map(Number);
    return new Intl.DateTimeFormat(locale(), { timeStyle: 'short' }).format(new Date(2000, 0, 1, h, m));
  },
  interval: (v) => `${v.start ? dateText(v.start) : '…'} – ${v.end ? dateText(v.end) : '…'}`,
  money: (v) => number(v.amount, { style: 'currency', currency: v.currency }),
  percentage: (v) => number(v, { style: 'percent', maximumFractionDigits: 2 }),
  number: (v) => number(v),
  integer: (v) => number(v),
  decimal: (v) => number(v),
  group4: (v) => v.replace(/(.{4})(?=.)/g, '$1 '),
  boolean: (v) => (v ? 'Yes' : 'No'),
  name: (v) => v.full,
  address: (v) => [...(v.lines ?? []), [v.postcode, v.locality].filter(Boolean).join(' '), country(v.country)].filter(Boolean).join(', '),
  country,
  postcode: (v) => v.code,
  geo: (v) => `${v.lat}, ${v.lon}`,
  quantity: (v) => `${number(v.value)} ${v.unit}`,
  card: (v) => `${v.brand} ending ${v.last4}`,
  mask: (v) => `ending ${String(v).slice(-4)}`,
  file: (v) => v.name,
};

export function formatValue(spec, field, v) {
  if (v == null || v === '') return '';
  if (Array.isArray(v)) return v.map((x) => formatValue(spec, { ...field, list: false }, x)).join(', ');
  const f = formats[kind(spec, field)];
  try {
    if (f) return f(v);
  } catch {}
  return typeof v === 'object' ? Object.values(v).filter((x) => x != null).join(' ') : String(v);
}

export function editText(spec, field, v) {
  if (v == null) return '';
  if (Array.isArray(v)) return v.map((x) => editText(spec, { ...field, list: false }, x)).join(', ');
  switch (kind(spec, field)) {
    case 'money':
      return `${v.amount} ${v.currency}`;
    case 'name':
      return v.full;
    case 'address':
      return formats.address(v);
    case 'percentage':
      return `${Number(v) * 100}%`;
  }
  return typeof v === 'object' ? JSON.stringify(v) : String(v);
}

export function confidenceWord(c) {
  if (c >= 0.95) return ['Certain', 'success'];
  if (c >= 0.7) return ['Likely', 'warning'];
  return ['Unsure', 'destructive'];
}
