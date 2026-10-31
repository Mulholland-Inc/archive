export class Unauthorized extends Error {}

export const API = location.hostname.startsWith('app.') ? location.origin.replace('//app.', '//api.') : '';

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

export function workspace() {
  try {
    return localStorage.workspace ?? '';
  } catch {
    return '';
  }
}

export function setWorkspace(id) {
  try {
    if (id) localStorage.workspace = id;
    else delete localStorage.workspace;
  } catch {}
}

export const passkeys = typeof PublicKeyCredential !== 'undefined';

function devices() {
  try {
    return JSON.parse(localStorage.passkeys ?? '{}');
  } catch {
    return {};
  }
}

export const devicePasskey = () => Object.values(devices()).includes('yes');

export const passkeyAsked = (user) => user in devices();

export function rememberPasskey(user, answer) {
  try {
    localStorage.passkeys = JSON.stringify({ ...devices(), [user]: answer });
  } catch {}
}

const bytes = (s) => Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(s.length / 4) * 4, '=')), (c) => c.charCodeAt(0));
const text = (b) => btoa(String.fromCharCode(...new Uint8Array(b))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

export async function passkeyLogin() {
  const pk = (await api('/auth/passkey/login', { method: 'POST' })).publicKey;
  pk.challenge = bytes(pk.challenge);
  pk.allowCredentials = (pk.allowCredentials ?? []).map((c) => ({ ...c, id: bytes(c.id) }));
  const cred = await navigator.credentials.get({ publicKey: pk });
  const r = cred.response;
  const done = await api('/auth/passkey/login/finish', {
    method: 'POST',
    body: {
      id: cred.id,
      rawId: text(cred.rawId),
      type: cred.type,
      response: { clientDataJSON: text(r.clientDataJSON), authenticatorData: text(r.authenticatorData), signature: text(r.signature), userHandle: r.userHandle ? text(r.userHandle) : null }
    }
  });
  rememberPasskey(done.user.id, 'yes');
}

export async function passkeyRegister(user) {
  const pk = (await api('/auth/passkey/register', { method: 'POST' })).publicKey;
  pk.challenge = bytes(pk.challenge);
  pk.user.id = bytes(pk.user.id);
  pk.excludeCredentials = (pk.excludeCredentials ?? []).map((c) => ({ ...c, id: bytes(c.id) }));
  const cred = await navigator.credentials.create({ publicKey: pk });
  const r = cred.response;
  await api('/auth/passkey/register/finish', {
    method: 'POST',
    body: {
      id: cred.id,
      rawId: text(cred.rawId),
      type: cred.type,
      response: { clientDataJSON: text(r.clientDataJSON), attestationObject: text(r.attestationObject), transports: r.getTransports?.() ?? [] }
    }
  });
  rememberPasskey(user, 'yes');
}

export async function send(path, { method = 'GET', body, raw, signal, workspace: named } = {}) {
  const init = { method, headers: {}, signal, credentials: 'include' };
  if (named || workspace()) init.headers['Archive-Workspace'] = named || workspace();
  if (space()) init.headers['Archive-Space'] = space();
  if (raw !== undefined) init.body = raw;
  else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers['Content-Type'] = 'application/json';
  }
  const r = await fetch(API + path, init);
  if (r.status === 401) throw new Unauthorized();
  if (!r.ok && r.status !== 204) {
    const out = await r.json().catch(() => ({}));
    throw Object.assign(new Error(out.error ?? r.statusText), { status: r.status });
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

export function setupHidden() {
  try {
    return localStorage[`setup:${workspace()}`] === 'hidden';
  } catch {
    return false;
  }
}

export function hideSetup() {
  try {
    localStorage[`setup:${workspace()}`] = 'hidden';
  } catch {}
}

export function devMode() {
  try {
    return localStorage.dev === 'on';
  } catch {
    return false;
  }
}

export function setDevMode(on) {
  try {
    if (on) localStorage.dev = 'on';
    else delete localStorage.dev;
  } catch {}
}

export function connectedApps() {
  try {
    return JSON.parse(localStorage[`connected:${workspace()}`] ?? '[]');
  } catch {
    return [];
  }
}
