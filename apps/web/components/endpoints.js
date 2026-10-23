const part = encodeURIComponent;

export const endpoints = {
  types: '/api/types',
  records: (type) => `/api/${part(type)}`,
  record: (type, id) => `/api/${part(type)}/${part(id)}`,
  provenance: (type, id) => `/api/${part(type)}/${part(id)}/provenance`,
  changes: '/api/changes',
  preview: (id) => `/api/changes/${part(id)}/preview`,
  apply: (id) => `/api/changes/${part(id)}/apply`,
  revert: (id) => `/api/changes/${part(id)}/revert`,
  search: '/api/search',
  sources: '/api/sources',
  source: (id) => `/api/sources/${part(id)}`,
  runSource: (id) => `/api/sources/${part(id)}/run`,
  views: '/api/views',
  view: (id) => `/api/views/${part(id)}`,
  files: '/api/files',
  file: (id) => `/api/files/${part(id)}`,
  fileContent: (id) => `/api/files/${part(id)}/content`,
  fileThumbnail: (id) => `/api/files/${part(id)}/thumbnail`,
  fileText: (id) => `/api/files/${part(id)}/text`,
  fileLink: (id) => `/api/files/${part(id)}/link`,
  tus: '/api/files/tus',
  methods: '/auth/methods',
  magic: '/auth/magic',
  setup: '/auth/setup',
  me: '/auth/me',
  signOut: '/auth/signout',
  spaces: '/auth/spaces',
  members: (space) => `/auth/spaces/${part(space)}/members`,
  tokens: '/auth/tokens',
  token: (id) => `/auth/tokens/${part(id)}`,
  passkeyRegister: '/auth/passkey/register',
  passkeyRegisterFinish: '/auth/passkey/register/finish',
  passkeyLogin: '/auth/passkey/login',
  passkeyLoginFinish: '/auth/passkey/login/finish',
  oidc: (provider) => `/auth/oidc/${part(provider)}`,
};

const page = (name) => new URL(`../app/${name}`, import.meta.url).pathname;

const withQuery = (name, params) => {
  const q = new URLSearchParams(Object.entries(params).filter(([, v]) => v != null && v !== ''));
  return q.size ? `${page(name)}?${q}` : page(name);
};

export const pages = {
  home: page(''),
  signIn: page('signin.html'),
  records: (type, params = {}) => withQuery('records.html', { type, ...params }),
  record: (type, id) => withQuery('record.html', { type, id }),
  schema: (type) => withQuery('schema.html', { type }),
  changes: page('changes.html'),
  settings: page('settings.html'),
  search: (params = {}) => withQuery('search.html', params),
  files: (params = {}) => withQuery('files.html', params),
  file: (params) => withQuery('file.html', params),
  sources: page('sources.html'),
};
