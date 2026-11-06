
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
        const now = Date.now() / 1000;
        for (const ws of this.ctx.getWebSockets()) {
          try {
            if (ws.deserializeAttachment()?.exp > now) ws.send(frame);
            else ws.close(1000, 'ticket expired');
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
      pair[1].serializeAttachment({ exp: Number(request.headers.get('X-Archive-Expires')) });
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

const fromBase64 = (s) => Uint8Array.from(atob(s.replaceAll('-', '+').replaceAll('_', '/')), (c) => c.charCodeAt(0));

async function ticketOf(ticket, env) {
  const [claims, sig] = ticket.split('.');
  if (!env.LIVE_KEY || !claims || !sig) return null;
  const encode = (s) => new TextEncoder().encode(s);
  try {
    const key = await crypto.subtle.importKey('raw', encode(env.LIVE_KEY), { name: 'HMAC', hash: 'SHA-256' }, false, ['verify']);
    if (!(await crypto.subtle.verify('HMAC', key, fromBase64(sig), encode(`live ticket ${claims}`)))) return null;
    const t = JSON.parse(new TextDecoder().decode(fromBase64(claims)));
    return t.exp > Date.now() / 1000 && WORKSPACE.test(t.workspace) && t.user ? t : null;
  } catch {
    return null;
  }
}

async function connect(request, env, url) {
  if (request.headers.get('Upgrade') !== 'websocket') return new Response('want a WebSocket', { status: 426 });
  const api = apiOrigin(url, env);
  const origin = request.headers.get('Origin');
  if (origin && origin !== api.replace('//api.', '//app.') && origin !== env.APP_ORIGIN) {
    return new Response('this origin may not listen', { status: 403 });
  }
  const t = await ticketOf(url.searchParams.get('ticket') || '', env);
  if (!t) return new Response('want a ticket from /api/boot?only=live', { status: 401 });
  const headers = new Headers(request.headers);
  headers.set('X-Archive-User', t.user);
  headers.set('X-Archive-Workspace', t.workspace);
  headers.set('X-Archive-Expires', String(t.exp));
  return stream(env, t.workspace).fetch(new Request('https://stream/connect', { headers }));
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (url.pathname === '/live/announce') return announce(request, env);
    if (url.pathname === '/live') return connect(request, env, url);
    if (url.hostname.startsWith('api.')) return fetch(request);
    const asset = await env.ASSETS.fetch(request);
    const missing = url.pathname.startsWith('/assets/') && (asset.headers.get('Content-Type') || '').startsWith('text/html');
    return missing ? new Response('not found', { status: 404 }) : asset;
  }
};
