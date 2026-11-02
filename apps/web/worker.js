// The app Worker: the web app's static files, and live data at the edge (docs/adr/0003-hosting.md).
//
// The Worker serves app.<domain> and, by a route Terraform makes, api.<domain>/live*, so the browser's socket
// to api.<domain>/live carries the API's own session cookie. Two routes run here; everything else is the
// static files (or, on the API host, passes on to the API):
//
//   GET  /live?workspace=<id>  a browser's WebSocket. The Worker asks the API whether the cookie's session may
//                              read that workspace (GET /auth/live), and only then hands the upgrade to the
//                              workspace's Durable Object.
//   POST /live/announce        the API saying what a write moved, {workspace, changes: [{table, id}], drop},
//                              with the shared key; relayed to the workspace's Durable Object.
//
// WorkspaceStream is one Durable Object per workspace. It accepts sockets with the hibernation API, so a socket
// with nothing to hear costs nothing, and the object sleeps between announces. It sends each socket one
// {event:"ready", workspace} and then {event:"change", changes} per announce: what moved, never row data; the
// browser re-reads through the API, so row-level security decides what it sees. A socket may send "ping",
// answered "pong" without waking the object; other frames are for later (presence, drafts) and ignored now.
//
// Nothing here is on a timer and nothing sweeps: a change is announced only because something moved. A
// browser that was disconnected re-reads what it has on screen once when it is back (src/lib/stream.svelte.js),
// not on every connect, so a socket that flaps cannot turn into a storm of reads. `archive serve` holds the
// same sockets with the same frames itself when no Worker is in front of it (src/live.go).

const WORKSPACE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

// Sockets one workspace holds at once: so a client in a dial loop cannot grow one object without bound.
const MOST_SOCKETS = 512;

// The largest announce relayed, in bytes; Go sends at most 20 ids per table.
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
      // A user who left the workspace stops hearing it.
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
      // The user is the socket's tag, so a drop finds their sockets after the object hibernated.
      this.ctx.acceptWebSocket(pair[1], user ? [user] : []);
      pair[1].send(JSON.stringify({ event: 'ready', workspace: request.headers.get('X-Archive-Workspace') || '' }));
      return new Response(null, { status: 101, webSocket: pair[0] });
    }
    return new Response('not found', { status: 404 });
  }

  // Reserved for frames a browser sends later, like presence; nothing is heard yet but ping.
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

// The API's address: the request's own on api.<domain>, or app. made api. (ADR 0003). API_ORIGIN overrides
// it, for wrangler dev.
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
  // Only the web app's pages may open one with the browser's cookie; the cookie is SameSite=Lax as well.
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
    // The route on the API host is api.<domain>/live*: anything else it catches is the API's.
    if (url.hostname.startsWith('api.')) return fetch(request);
    return env.ASSETS.fetch(request);
  }
};
