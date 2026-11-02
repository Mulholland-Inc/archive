// The HTTP API. On app.<domain> it is api.<domain>; anywhere else (vite dev on :5173) it is the same origin,
// which vite proxies to `archive serve` on :8080. The session is the API host's own cookie, and the current
// workspace goes along in the Archive-Workspace header (docs/adr/0003-hosting.md).

export class Unauthorized extends Error {}

export const API = location.hostname.startsWith('app.') ? location.origin.replace('//app.', '//api.') : '';

function stored(key, fallback = '') {
  try {
    return localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

function store(key, value) {
  try {
    if (value === null || value === undefined || value === '') localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {}
}

export const workspace = () => stored('workspace');
export const setWorkspace = (id) => store('workspace', id);

export function local(key, fallback) {
  try {
    return JSON.parse(stored(key, '')) ?? fallback;
  } catch {
    return fallback;
  }
}

export const setLocal = (key, value) => store(key, JSON.stringify(value));

export async function send(path, { method = 'GET', body, raw, signal, workspace: named } = {}) {
  const init = { method, headers: {}, signal, credentials: 'include' };
  if (named || workspace()) init.headers['Archive-Workspace'] = named || workspace();
  if (raw !== undefined) init.body = raw;
  else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers['Content-Type'] = 'application/json';
  }
  const r = await fetch(API + path, init);
  if (r.status === 401) throw new Unauthorized('Signed out');
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

export async function* chatStream(body, signal) {
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

export async function upload(file, access) {
  const form = new FormData();
  form.append('file', file);
  return api(`/api/files${access ? `?access=${encodeURIComponent(access)}` : ''}`, { method: 'POST', raw: form });
}

export const fileURL = (id, part = 'content') => `${API}/api/files/${id}/${part}`;

// An <img> or a link can't send Archive-Workspace, so file bytes are fetched with it and shown from a blob.
const blobs = new Map();
export function blobURL(path, named) {
  const key = `${named || workspace()}:${path}`;
  if (!blobs.has(key)) {
    const p = send(path, { workspace: named }).then(async (r) => URL.createObjectURL(await r.blob()));
    p.catch(() => blobs.delete(key));
    blobs.set(key, p);
  }
  return blobs.get(key);
}

export async function signOut() {
  await api('/auth/signout', { method: 'POST' }).catch(() => {});
  setWorkspace('');
}

// Passkeys: one per device, discoverable, so sign-in needs no email. The app remembers per user whether it
// asked to add one on this device, and which users have one here, so a returning device offers it first.

export const passkeys = typeof PublicKeyCredential !== 'undefined';

const devices = () => local('passkeys', {});
export const devicePasskey = () => Object.values(devices()).includes('yes');
export const passkeyAsked = (user) => user in devices();
export const rememberPasskey = (user, answer) => setLocal('passkeys', { ...devices(), [user]: answer });

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
