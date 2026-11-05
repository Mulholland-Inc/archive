
const WORKSPACE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

const MOST_SOCKETS = 512;

const MOST_ANNOUNCE = 65536;

export class WorkspaceStream {
  constructor(ctx) {
    this.ctx = ctx;
    ctx.setWebSocketAutoResponse(new WebSocketRequestResponsePair('ping', 'pong'));
  }

  async fetch(request) {
    const url = new URL(request.url);
    if (url.pathname === '/announce') {
      let said = {};
      try {
        said = JSON.parse(await request.text()) || {};
      } catch {}
      for (const user of Array.isArray(said.drop) ? said.drop : []) {
        for (const ws of this.ctx.getWebSockets(String(user))) {
          try {
            ws.close(4003, 'left the workspace');
          } catch {}
        }
      }
      const changes = (Array.isArray(said.changes) ? said.changes : [])
        .filter((c) => c && typeof c.table === 'string' && c.table)
        .map((c) => (typeof c.id === 'string' && c.id ? { table: c.table, id: c.id } : { table: c.table }));
      if (changes.length) {
        const frame = JSON.stringify({ event: 'change', changes });
        for (const ws of this.ctx.getWebSockets()) {
          try {
            ws.send(frame);
          } catch {}
        }
      }
      return new Response(null, { status: 204 });
    }
    if (url.pathname === '/connect' && request.headers.get('Upgrade') === 'websocket') {
      if (this.ctx.getWebSockets().length >= MOST_SOCKETS) return new Response('too many listeners', { status: 429 });
      const user = request.headers.get('X-Archive-User') || '';
      const pair = new WebSocketPair();
      this.ctx.acceptWebSocket(pair[1], user ? [user] : []);
      pair[1].send(JSON.stringify({ event: 'ready', workspace: request.headers.get('X-Archive-Workspace') || '' }));
      return new Response(null, { status: 101, webSocket: pair[0] });
    }
    return new Response('not found', { status: 404 });
  }

  webSocketMessage() {}

  webSocketClose(ws, code, reason) {
    try {
      ws.close(code === 1005 || code === 1006 ? 1000 : code, reason);
    } catch {}
  }

  webSocketError(ws) {
    try {
      ws.close(1011, 'error');
    } catch {}
  }
}

function apiOrigin(url, env) {
  if (env.API_ORIGIN) return env.API_ORIGIN.replace(/\/$/, '');
  return url.hostname.startsWith('app.') ? url.origin.replace('//app.', '//api.') : url.origin;
}

async function sameKey(got, want) {
  const a = new TextEncoder().encode(got);
  const b = new TextEncoder().encode(want);
  return a.byteLength === b.byteLength && crypto.subtle.timingSafeEqual(a, b);
}

const stream = (env, workspace) => env.WORKSPACE_STREAM.get(env.WORKSPACE_STREAM.idFromName(workspace));

async function announce(request, env) {
  if (request.method !== 'POST') return new Response('POST an announce', { status: 405 });
  if (!env.LIVE_KEY || !(await sameKey(request.headers.get('Authorization') || '', `Bearer ${env.LIVE_KEY}`))) {
    return new Response('unauthorized', { status: 401 });
  }
  const body = await request.text();
  if (body.length > MOST_ANNOUNCE) return new Response('too large', { status: 413 });
  let workspace = '';
  try {
    workspace = JSON.parse(body).workspace || '';
  } catch {}
  if (!WORKSPACE.test(workspace)) return new Response('want {workspace, changes}', { status: 400 });
  return stream(env, workspace).fetch('https://stream/announce', { method: 'POST', body });
}

async function connect(request, env, url) {
  if (request.headers.get('Upgrade') !== 'websocket') return new Response('want a WebSocket', { status: 426 });
  const workspace = url.searchParams.get('workspace') || '';
  if (!WORKSPACE.test(workspace)) return new Response('want ?workspace=<id>', { status: 400 });
  const api = apiOrigin(url, env);
  const origin = request.headers.get('Origin');
  if (origin && origin !== api.replace('//api.', '//app.') && origin !== env.APP_ORIGIN) {
    return new Response('this origin may not listen', { status: 403 });
  }
  const check = await fetch(`${api}/auth/live`, {
    headers: { Cookie: request.headers.get('Cookie') || '', 'Archive-Workspace': workspace, Accept: 'application/json' }
  });
  if (!check.ok || !(check.headers.get('Content-Type') || '').startsWith('application/json')) {
    return new Response(check.ok ? 'no answer from the API' : await check.text(), { status: check.ok ? 502 : check.status });
  }
  const who = await check.json();
  if (who.workspace !== workspace || !who.user) return new Response('forbidden', { status: 403 });
  const headers = new Headers(request.headers);
  headers.set('X-Archive-User', who.user);
  headers.set('X-Archive-Workspace', workspace);
  return stream(env, workspace).fetch(new Request('https://stream/connect', { headers }));
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (url.pathname === '/live/announce') return announce(request, env);
    if (url.pathname === '/live') return connect(request, env, url);
    if (url.hostname.startsWith('api.')) return fetch(request);
    return env.ASSETS.fetch(request);
  }
};
