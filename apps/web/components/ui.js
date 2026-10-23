import { locale } from './format.js';

export const field = (text, control, id) => {
  const wrap = document.createElement('div');
  wrap.dataset.formField = '';
  const l = document.createElement('label');
  l.textContent = text;
  if (id) l.htmlFor = id;
  wrap.append(l, control);
  return wrap;
};

export const textInput = (id, value = '', placeholder = '') => {
  const input = document.createElement('input');
  input.type = 'text';
  input.id = id;
  input.autocomplete = 'off';
  input.value = value;
  if (placeholder) input.placeholder = placeholder;
  return input;
};

export const select = (aria, options, value) => {
  const s = document.createElement('app-select');
  s.setAttribute('aria-label', aria);
  s.options = options;
  s.value = value;
  return s;
};

export const toggle = (text, checked) => {
  const s = document.createElement('app-switch');
  s.setAttribute('label', text);
  s.checked = checked;
  return s;
};

export const button = (text, icon, kind = 'secondary') => {
  const b = document.createElement('button');
  b.type = 'button';
  b.className = `button button-${kind} button-sm`;
  b.innerHTML = `${icon ? `<app-icon name="${icon}"></app-icon>` : ''}<span></span>`;
  b.querySelector('span').textContent = text;
  return b;
};

export const link = (text, href, kind) => {
  const a = document.createElement('a');
  a.href = href;
  a.textContent = text;
  if (kind) a.className = `button button-${kind} button-sm`;
  return a;
};

export const el = (tag, text, attrs = {}) => {
  const e = document.createElement(tag);
  if (text != null) e.textContent = text;
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
};

export const header = (title, actions = [], about = []) => {
  const h = document.createElement('header');
  h.innerHTML = '<div><h1></h1></div><div></div>';
  h.querySelector('h1').textContent = title;
  h.firstElementChild.append(...about);
  h.lastElementChild.append(...actions);
  return h;
};

export const alertBox = () => {
  const p = el('p', '', { role: 'alert' });
  p.hidden = true;
  p.show = (err) => {
    p.textContent = err?.message ?? err ?? '';
    p.hidden = !p.textContent;
  };
  return p;
};

export async function copy(b, text) {
  const span = b.querySelector('span') ?? b;
  const label = span.textContent;
  try {
    await navigator.clipboard.writeText(text);
    span.textContent = 'Copied';
  } catch {
    span.textContent = 'Select and copy it';
  }
  setTimeout(() => (span.textContent = label), 1800);
}

export const when = (iso, style = { dateStyle: 'medium', timeStyle: 'short' }) =>
  iso ? new Intl.DateTimeFormat(locale(), style).format(new Date(iso)) : '';

export const bytes = (n) => {
  const units = ['byte', 'kilobyte', 'megabyte', 'gigabyte', 'terabyte'];
  const i = n > 0 ? Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1000))) : 0;
  return new Intl.NumberFormat(locale(), { style: 'unit', unit: units[i], unitDisplay: i ? 'short' : 'long', maximumFractionDigits: 1 }).format(n / 1000 ** i);
};

export const roles = ['owner', 'admin', 'editor', 'viewer'];

export const roleLabel = (r) => r.replace(/^./, (c) => c.toUpperCase());

export function snippet(text) {
  const p = document.createElement('p');
  p.dataset.snippet = '';
  String(text ?? '')
    .split('**')
    .forEach((part, i) => p.append(i % 2 ? el('strong', part) : part));
  return p;
}

export function listItem(main, aside = []) {
  const li = document.createElement('li');
  const body = document.createElement('div');
  body.append(...main.filter(Boolean));
  const end = document.createElement('div');
  end.dataset.actions = '';
  end.append(...aside.filter(Boolean));
  li.append(body, end);
  return li;
}
