
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


const DRAFT = 'draft:';
let drafts = Promise.resolve();

const queue = (work) => (drafts = drafts.then(work).catch(() => ''));

const pipe = async (bytes, stream) => new Uint8Array(await new Response(new Blob([bytes]).stream().pipeThrough(stream)).arrayBuffer());

export const saveDraft = (id, text) => queue(async () => {
  const bytes = text ? await pipe(new TextEncoder().encode(text), new CompressionStream('deflate-raw')) : [];
  store(DRAFT + id, Array.from(bytes, (b) => String.fromCharCode(b)).join(''));
});

export const loadDraft = (id) => queue(async () => {
  const packed = stored(DRAFT + id);
  return packed && new TextDecoder().decode(await pipe(Uint8Array.from(packed, (c) => c.charCodeAt(0)), new DecompressionStream('deflate-raw')));
});

const clearDrafts = () => queue(() => {
  for (const key of Object.keys(localStorage)) if (key.startsWith(DRAFT)) store(key, '');
});

export async function send(path, { method = 'GET', body, raw, signal, headers = {}, workspace: named } = {}) {
  const init = { method, headers: { ...headers }, signal, credentials: 'include' };
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

export async function* turnStream(body, signal) {
  const r = await send('/api/turn', { method: 'POST', body, signal });
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

export async function upload(file, access, parent) {
  const form = new FormData();
  form.append('file', file);
  const q = new URLSearchParams();
  if (access) q.set('access', access);
  if (parent) q.set('parent', parent);
  return api(`/api/files${q.size ? `?${q}` : ''}`, { method: 'POST', raw: form });
}

// Uploads a file of any size by tus, in parts small enough for one request each, and picks up where it left off
// after a dropped connection. Resolves to the new file's id.
const TUS = { 'Tus-Resumable': '1.0.0' };
const PART = 16 << 20;

export async function uploadLarge(file, progress = () => {}) {
  const meta = `filename ${btoa(Array.from(new TextEncoder().encode(file.name), (b) => String.fromCharCode(b)).join(''))}`;
  const made = await send('/api/files/tus', { method: 'POST', headers: { ...TUS, 'Upload-Length': String(file.size), 'Upload-Metadata': meta } });
  const done = made.headers.get('Archive-File-Id');
  if (done) return done;
  const at = new URL(made.headers.get('Location'), location.origin).pathname;
  let off = 0;
  for (let tries = 0; ; ) {
    try {
      const r = await send(at, { method: 'PATCH', headers: { ...TUS, 'Upload-Offset': String(off), 'Content-Type': 'application/offset+octet-stream' }, raw: file.slice(off, off + PART) });
      off = Number(r.headers.get('Upload-Offset'));
      progress(off / file.size);
      const id = r.headers.get('Archive-File-Id');
      if (id) return id;
      tries = 0;
    } catch (e) {
      if (++tries > 5 || e instanceof Unauthorized) throw e;
      await new Promise((ok) => setTimeout(ok, 1000 * 2 ** tries));
      off = Number((await send(at, { method: 'HEAD', headers: TUS })).headers.get('Upload-Offset'));
    }
  }
}

export const fileURL = (id, part = 'content') => `${API}/api/files/${id}/${part}`;

const blobs = new Map();
export function forgetBlobs(prefix) {
  for (const [key, p] of blobs) {
    if (!key.slice(key.indexOf(':') + 1).startsWith(prefix)) continue;
    blobs.delete(key);
    p.then((u) => URL.revokeObjectURL(u)).catch(() => {});
  }
}

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
  clearDrafts();
}


export const said = (e) => e.message.replace(/: (bad request|busy|conflict|forbidden)$/, '');

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
  if (done.user) rememberPasskey(done.user.id, 'yes');
  return done;
}

export async function passkeyVerify() {
  const pk = (await api('/auth/passkey/verify', { method: 'POST' })).publicKey;
  pk.challenge = bytes(pk.challenge);
  pk.allowCredentials = (pk.allowCredentials ?? []).map((c) => ({ ...c, id: bytes(c.id) }));
  const cred = await navigator.credentials.get({ publicKey: pk });
  const r = cred.response;
  await api('/auth/passkey/verify/finish', {
    method: 'POST',
    body: {
      id: cred.id,
      rawId: text(cred.rawId),
      type: cred.type,
      response: { clientDataJSON: text(r.clientDataJSON), authenticatorData: text(r.authenticatorData), signature: text(r.signature), userHandle: r.userHandle ? text(r.userHandle) : null }
    }
  });
}

export async function passkeyRegister(user) {
  const pk = (await api('/auth/passkey/register', { method: 'POST' })).publicKey;
  pk.challenge = bytes(pk.challenge);
  pk.user.id = bytes(pk.user.id);
  pk.excludeCredentials = (pk.excludeCredentials ?? []).map((c) => ({ ...c, id: bytes(c.id) }));
  const cred = await navigator.credentials.create({ publicKey: pk });
  const r = cred.response;
  const done = await api('/auth/passkey/register/finish', {
    method: 'POST',
    body: {
      id: cred.id,
      rawId: text(cred.rawId),
      type: cred.type,
      response: { clientDataJSON: text(r.clientDataJSON), attestationObject: text(r.attestationObject), transports: r.getTransports?.() ?? [] }
    }
  });
  rememberPasskey(user, 'yes');
  return done;
}
