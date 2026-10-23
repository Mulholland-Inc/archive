import type {
  ArchiveFile,
  ArchiveRecord,
  Change,
  ChangePreview,
  Me,
  Member,
  Ontology,
  Op,
  Proposal,
  ProposalStatus,
  Provenance,
  ReviewOutcome,
  RecordLink,
  SearchHit,
  SigninMethods,
  Source,
  Space,
  SyncResult,
  Token,
  View,
  ViewInput,
  ModuleApp
} from './types';

export type FieldError = { path: string; message: string };

export class ApiError extends Error {
  status: number;
  fields: FieldError[];

  constructor(status: number, message: string, fields: FieldError[] = []) {
    super(message);
    this.status = status;
    this.fields = fields;
  }
}

let onUnauthorized: () => void = () => {};

export function setUnauthorizedHandler(f: () => void) {
  onUnauthorized = f;
}

async function errorFrom(res: Response): Promise<ApiError> {
  let body: { error?: string; type?: string; errors?: FieldError[] } = {};
  try {
    body = await res.json();
  } catch {
    body = {};
  }
  if (body.errors) {
    const first = body.errors[0];
    const message = first ? `${first.path}: ${first.message}` : `Invalid ${body.type ?? 'input'}`;
    return new ApiError(res.status, message, body.errors);
  }
  return new ApiError(res.status, body.error ?? res.statusText ?? 'Request failed');
}

type Query = { [key: string]: string | number | boolean | undefined | null };

function withQuery(path: string, query?: Query) {
  if (!query) return path;
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== null && v !== '') q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `${path}?${s}` : path;
}

const cache = new Map<string, { at: number; value: Promise<unknown> }>();
const fresh = 20_000;

function cached<T>(path: string, query?: Query): Promise<T> {
  const key = withQuery(path, query);
  const hit = cache.get(key);
  if (hit && Date.now() - hit.at < fresh) return hit.value as Promise<T>;
  const value = request<T>('GET', path, undefined, query);
  cache.set(key, { at: Date.now(), value });
  value.catch(() => cache.delete(key));
  return value;
}

function prime(path: string, value: unknown) {
  cache.set(path, { at: Date.now(), value: Promise.resolve(value) });
}

async function request<T>(method: string, path: string, body?: unknown, query?: Query, quiet = false): Promise<T> {
  if (method !== 'GET') cache.clear();
  const init: RequestInit = { method, credentials: 'same-origin', headers: { Accept: 'application/json' } };
  if (body instanceof FormData) {
    init.body = body;
  } else if (body !== undefined) {
    init.body = JSON.stringify(body);
    (init.headers as { [k: string]: string })['Content-Type'] = 'application/json';
  }
  let res: Response;
  try {
    res = await fetch(withQuery(path, query), init);
  } catch {
    throw new ApiError(0, 'Archive is not reachable. Check that the server is running.');
  }
  if (res.status === 401 && !quiet) onUnauthorized();
  if (!res.ok) throw await errorFrom(res);
  if (res.status === 204) return undefined as T;
  const type = res.headers.get('Content-Type') ?? '';
  if (type.includes('application/json')) return (await res.json()) as T;
  return (await res.text()) as T;
}

const enc = encodeURIComponent;

export type ListOptions = { space?: string; where?: { [field: string]: string }; limit?: number; after?: string; reveal?: boolean };

export const api = {
  me: () => request<Me>('GET', '/auth/me', undefined, undefined, true),
  methods: () => request<SigninMethods>('GET', '/auth/methods'),
  magic: (email: string) => request<{ status: string }>('POST', '/auth/magic', { email }),
  setup: (token: string, email: string, name: string) => request<{ user: unknown }>('POST', '/auth/setup', { token, email, name }),
  signout: () => request<void>('POST', '/auth/signout'),
  passkeyLoginBegin: () => request<CredentialRequestJSON>('POST', '/auth/passkey/login'),
  passkeyLoginFinish: (credential: unknown) => request<{ user: unknown }>('POST', '/auth/passkey/login/finish', credential),
  passkeyRegisterBegin: () => request<CredentialCreationJSON>('POST', '/auth/passkey/register'),
  passkeyRegisterFinish: (credential: unknown) => request<{ passkeys: number }>('POST', '/auth/passkey/register/finish', credential),

  spaces: () => request<Space[]>('GET', '/auth/spaces'),
  addSpace: (name: string) => request<Space>('POST', '/auth/spaces', { name }),
  removeSpace: (name: string) => request<void>('DELETE', `/auth/spaces/${enc(name)}`),
  members: (space: string) => request<Member[]>('GET', `/auth/spaces/${enc(space)}/members`),
  setMember: (space: string, email: string, role: string) => request<Member[]>('POST', `/auth/spaces/${enc(space)}/members`, { email, role }),
  tokens: () => request<Token[]>('GET', '/auth/tokens'),
  createToken: (body: { name: string; spaces?: { [space: string]: string }; expires?: string; full?: boolean }) =>
    request<Token>('POST', '/auth/tokens', body),
  revokeToken: (id: string) => request<void>('DELETE', `/auth/tokens/${enc(id)}`),

  ontology: () => request<Ontology>('GET', '/api/types'),
  apps: () => request<ModuleApp[]>('GET', '/api/apps', undefined, undefined, true),
  changes: () => request<Change[]>('GET', '/api/changes'),
  propose: (ops: Op[]) => request<Change>('POST', '/api/changes', { ops }),
  preview: (id: string) => request<ChangePreview>('POST', `/api/changes/${enc(id)}/preview`),
  apply: (id: string) => request<Change>('POST', `/api/changes/${enc(id)}/apply`),
  revert: (id: string) => request<Change>('POST', `/api/changes/${enc(id)}/revert`),

  list: (type: string, o: ListOptions = {}) =>
    cached<ArchiveRecord[]>(`/api/${enc(type)}`, {
      ...(o.where ?? {}),
      space: o.space,
      limit: o.limit,
      after: o.after,
      reveal: o.reveal || undefined
    }).then((rs) => {
      if (!o.reveal) for (const r of rs) if (!cache.has(`/api/${enc(r.type)}/${enc(r.id)}`)) prime(`/api/${enc(r.type)}/${enc(r.id)}`, r);
      return rs;
    }),
  get: (type: string, id: string, reveal = false) =>
    reveal
      ? request<ArchiveRecord>('GET', `/api/${enc(type)}/${enc(id)}`, undefined, { reveal: true })
      : cached<ArchiveRecord>(`/api/${enc(type)}/${enc(id)}`),
  create: (type: string, data: { [field: string]: unknown }, space?: string) =>
    request<ArchiveRecord>('POST', `/api/${enc(type)}`, data, { space }),
  patch: (type: string, id: string, data: { [field: string]: unknown }) => request<ArchiveRecord>('PATCH', `/api/${enc(type)}/${enc(id)}`, data),
  remove: (type: string, id: string) => request<void>('DELETE', `/api/${enc(type)}/${enc(id)}`),
  provenance: (type: string, id: string) => cached<Provenance[] | null>(`/api/${enc(type)}/${enc(id)}/provenance`),
  links: (type: string, id: string) => cached<RecordLink[]>(`/api/${enc(type)}/${enc(id)}/links`),

  search: (q: string, f: { type?: string; space?: string; source?: string; from?: string; to?: string; limit?: number } = {}) =>
    request<SearchHit[] | null>('GET', '/api/search', undefined, { q, ...f }),

  files: (o: { space?: string; limit?: number; after?: string } = {}) => request<ArchiveFile[] | null>('GET', '/api/files', undefined, o),
  file: (id: string) => request<ArchiveFile>('GET', `/api/files/${enc(id)}`),
  fileText: (id: string) => request<string>('GET', `/api/files/${enc(id)}/text`),
  fileLink: (id: string, expires = '24h') => request<{ url: string; expires_at: string }>('POST', `/api/files/${enc(id)}/link`, undefined, { expires }),
  fileContentURL: (id: string) => `/api/files/${enc(id)}/content`,
  fileThumbnailURL: (id: string) => `/api/files/${enc(id)}/thumbnail`,

  sources: () => request<Source[] | null>('GET', '/api/sources'),
  source: (id: string) => request<Source>('GET', `/api/sources/${enc(id)}`),
  addSource: (body: {
    kind: string;
    name: string;
    space?: string;
    type?: string;
    schedule?: string;
    rate?: number;
    config?: { [k: string]: unknown };
    credentials?: { [k: string]: unknown };
  }) => request<Source>('POST', '/api/sources', body),
  patchSource: (id: string, body: { type?: string; schedule?: string; rate?: number }) => request<Source>('PATCH', `/api/sources/${enc(id)}`, body),
  runSource: (id: string) => request<SyncResult>('POST', `/api/sources/${enc(id)}/run`),
  removeSource: (id: string) => request<void>('DELETE', `/api/sources/${enc(id)}`),

  demo: () => request<{ [type: string]: number }>('POST', '/api/demo'),
  review: (o: { status?: ProposalStatus; type?: string; space?: string; limit?: number; after?: string } = {}) => request<Proposal[]>('GET', '/api/review', undefined, o),
  reviewCounts: (space?: string) => request<{ [status: string]: number }>('GET', '/api/review/counts', undefined, { space }),
  proposal: (id: string) => request<Proposal>('GET', `/api/review/${enc(id)}`),
  approve: (ids: string[], edits?: { [id: string]: { [field: string]: unknown } }) => request<ReviewOutcome>('POST', '/api/review/approve', { ids, edits }),
  reject: (ids: string[]) => request<ReviewOutcome>('POST', '/api/review/reject', { ids }),

  views: () => request<View[]>('GET', '/api/views'),
  view: (id: string) => cached<View>(`/api/views/${enc(id)}`),
  addView: (v: ViewInput) => request<View>('POST', '/api/views', v),
  setView: (id: string, v: ViewInput) => request<View>('PUT', `/api/views/${enc(id)}`, v),
  removeView: (id: string) => request<void>('DELETE', `/api/views/${enc(id)}`)
};

export function upload(
  file: File,
  o: { space?: string; intake?: string },
  progress: (fraction: number) => void
): Promise<ArchiveFile> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', withQuery('/api/files', { space: o.space, intake: o.intake }));
    xhr.setRequestHeader('Accept', 'application/json');
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) progress(e.loaded / e.total);
    };
    xhr.onload = () => {
      let body: { error?: string; errors?: FieldError[] } & Partial<ArchiveFile> = {};
      try {
        body = JSON.parse(xhr.responseText);
      } catch {
        body = {};
      }
      if (xhr.status === 401) onUnauthorized();
      if (xhr.status >= 200 && xhr.status < 300) resolve(body as ArchiveFile);
      else reject(new ApiError(xhr.status, body.error ?? body.errors?.[0]?.message ?? `Upload failed (${xhr.status})`, body.errors ?? []));
    };
    xhr.onerror = () => reject(new ApiError(0, 'Upload failed. Check your connection.'));
    const form = new FormData();
    form.append('file', file, file.name);
    xhr.send(form);
  });
}

export type CredentialRequestJSON = { publicKey: { [k: string]: unknown } };
export type CredentialCreationJSON = { publicKey: { [k: string]: unknown } };

function fromB64url(s: string): ArrayBuffer {
  const b64 = s.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((s.length + 3) % 4);
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out.buffer;
}

function toB64url(buf: ArrayBuffer | null): string | null {
  if (!buf) return null;
  const bytes = new Uint8Array(buf);
  let s = '';
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

type Descriptor = { id: string; type: string; transports?: string[] };

function decodeRequest(o: CredentialRequestJSON): CredentialRequestOptions {
  const pk = o.publicKey as { challenge: string; allowCredentials?: Descriptor[] } & { [k: string]: unknown };
  return {
    publicKey: {
      ...(pk as object),
      challenge: fromB64url(pk.challenge),
      allowCredentials: pk.allowCredentials?.map((c) => ({ ...c, id: fromB64url(c.id) })) as PublicKeyCredentialDescriptor[]
    } as PublicKeyCredentialRequestOptions,
    mediation: 'optional'
  };
}

function decodeCreation(o: CredentialCreationJSON): CredentialCreationOptions {
  const pk = o.publicKey as {
    challenge: string;
    user: { id: string; name: string; displayName: string };
    excludeCredentials?: Descriptor[];
  } & { [k: string]: unknown };
  return {
    publicKey: {
      ...(pk as object),
      challenge: fromB64url(pk.challenge),
      user: { ...pk.user, id: fromB64url(pk.user.id) },
      excludeCredentials: pk.excludeCredentials?.map((c) => ({ ...c, id: fromB64url(c.id) })) as PublicKeyCredentialDescriptor[]
    } as PublicKeyCredentialCreationOptions
  };
}

function encodeCredential(c: PublicKeyCredential) {
  const r = c.response as AuthenticatorAttestationResponse & AuthenticatorAssertionResponse;
  const response: { [k: string]: unknown } = { clientDataJSON: toB64url(r.clientDataJSON) };
  if ('attestationObject' in r && r.attestationObject) {
    response.attestationObject = toB64url(r.attestationObject);
    response.transports = typeof r.getTransports === 'function' ? r.getTransports() : [];
  }
  if ('authenticatorData' in r && r.authenticatorData) {
    response.authenticatorData = toB64url(r.authenticatorData);
    response.signature = toB64url(r.signature);
    response.userHandle = toB64url(r.userHandle);
  }
  return {
    id: c.id,
    rawId: toB64url(c.rawId),
    type: c.type,
    authenticatorAttachment: c.authenticatorAttachment,
    clientExtensionResults: c.getClientExtensionResults(),
    response
  };
}

export function passkeysSupported() {
  return typeof window !== 'undefined' && 'PublicKeyCredential' in window && !!navigator.credentials;
}

export async function passkeySignIn() {
  const options = await api.passkeyLoginBegin();
  const cred = (await navigator.credentials.get(decodeRequest(options))) as PublicKeyCredential | null;
  if (!cred) throw new ApiError(0, 'No passkey was chosen.');
  return api.passkeyLoginFinish(encodeCredential(cred));
}

export async function passkeyRegister() {
  const options = await api.passkeyRegisterBegin();
  const cred = (await navigator.credentials.create(decodeCreation(options))) as PublicKeyCredential | null;
  if (!cred) throw new ApiError(0, 'No passkey was created.');
  return api.passkeyRegisterFinish(encodeCredential(cred));
}
