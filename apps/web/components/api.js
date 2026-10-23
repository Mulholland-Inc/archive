import { endpoints, pages } from './endpoints.js';

export const tidy = (message) => {
  const text = String(message).replace(/: (not found|conflict|forbidden|unauthorized|bad request)$/, '');
  return `${text.replace(/^./, (c) => c.toUpperCase())}${/[.?!]$/.test(text) ? '' : '.'}`;
};

export class ApiError extends Error {
  constructor(status, body) {
    const errors = body?.errors ?? [];
    super(tidy(body?.error ?? (errors.map((e) => (e.path ? `${e.path}: ${e.message}` : e.message)).join(', ') || `Request failed (${status})`)));
    this.status = status;
    this.fields = {};
    for (const e of errors) {
      const key = (e.path ?? '').split(/[.[]/)[0];
      (this.fields[key] ??= []).push(e.message);
    }
  }
}

export function toSignIn() {
  location.assign(`${pages.signIn}?${new URLSearchParams({ next: location.pathname + location.search })}`);
  return new Promise(() => {});
}

export async function api(path, { method = 'GET', body, query, redirect = true } = {}) {
  const url = new URL(path, location.origin);
  for (const [k, v] of Object.entries(query ?? {})) if (v != null && v !== '') url.searchParams.set(k, v);
  const res = await fetch(url, {
    method,
    credentials: 'same-origin',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 401 && redirect) return toSignIn();
  if (res.status === 204) return null;
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, data);
  return data;
}

let me;

export const loadMe = () => (me ??= api(endpoints.me));

export async function currentSpace() {
  const { spaces, home } = await loadMe();
  const saved = localStorage.getItem('archive.space');
  return spaces.find((s) => s.name === saved) ?? spaces.find((s) => s.id === home) ?? spaces[0] ?? null;
}

export const spaceLabel = (s, home) => (s.id === home ? 'Private' : s.name.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase()));

export function setSpace(name) {
  localStorage.setItem('archive.space', name);
}

export const spaceQuery = async () => ({ space: (await currentSpace())?.name });
