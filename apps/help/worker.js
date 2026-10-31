// Carries Help messages between the app and a private Discord forum, so nothing in the engine changes.
// Each conversation is one forum post. A signed ticket names the post, so only its sender can read the replies.
const LIMIT = 2000;
const encoder = new TextEncoder();
const base64url = (bytes) => btoa(String.fromCharCode(...new Uint8Array(bytes))).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '');

async function sign(secret, thread) {
  const key = await crypto.subtle.importKey('raw', encoder.encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
  return base64url(await crypto.subtle.sign('HMAC', key, encoder.encode(thread)));
}

async function ticketFor(secret, thread) {
  return `${thread}.${await sign(secret, thread)}`;
}

async function threadOf(secret, ticket = '') {
  const [thread, signature] = ticket.split('.');
  if (!/^\d+$/.test(thread ?? '') || !signature) return null;
  const want = encoder.encode(await sign(secret, thread));
  const have = encoder.encode(signature);
  if (want.length !== have.length) return null;
  return want.reduce((diff, b, i) => diff | (b ^ have[i]), 0) === 0 ? thread : null;
}

const line = (v, max) => String(v ?? '').replace(/\s+/g, ' ').trim().slice(0, max);

async function post(request, env, reply) {
  const body = await request.json().catch(() => null);
  const text = String(body?.text ?? '').trim();
  if (!text || text.length > LIMIT) return reply({ error: `want 1 to ${LIMIT} characters` }, 400);
  const thread = body.ticket ? await threadOf(env.HELP_SECRET, body.ticket) : null;
  if (body.ticket && !thread) return reply({ error: 'bad ticket' }, 403);

  const who = [line(body.name, 80), line(body.email, 120), line(body.archive, 80)].filter(Boolean).join(' · ');
  const message = { content: thread ? text : `**${who || 'Someone'}**\n${text}`.slice(0, LIMIT), allowed_mentions: { parse: [] } };
  if (!thread) message.thread_name = line(text, 90);
  const res = await fetch(`${env.DISCORD_WEBHOOK}?wait=true${thread ? `&thread_id=${thread}` : ''}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(message),
  });
  if (!res.ok) return reply({ error: 'could not reach Discord' }, 502);
  const sent = await res.json();
  return reply({ ticket: await ticketFor(env.HELP_SECRET, sent.channel_id), id: sent.id });
}

async function replies(request, env, reply) {
  const url = new URL(request.url);
  const thread = await threadOf(env.HELP_SECRET, url.searchParams.get('ticket') ?? '');
  if (!thread) return reply({ error: 'bad ticket' }, 403);
  const after = url.searchParams.get('after');
  const query = `limit=50${/^\d+$/.test(after ?? '') ? `&after=${after}` : ''}`;
  const res = await fetch(`${env.DISCORD_API ?? 'https://discord.com/api/v10'}/channels/${thread}/messages?${query}`, {
    headers: { Authorization: `Bot ${env.DISCORD_BOT_TOKEN}` },
  });
  if (!res.ok) return reply({ error: 'could not reach Discord' }, 502);
  const messages = await res.json();
  return reply(messages.filter((m) => !m.webhook_id && !m.author?.bot && m.content).reverse().map((m) => ({ id: m.id, name: m.author?.global_name ?? m.author?.username ?? 'Mulholland', text: m.content })));
}

export default {
  async fetch(request, env) {
    const origin = request.headers.get('Origin') ?? '';
    if (!env.ORIGINS.split(',').includes(origin)) return new Response('Forbidden', { status: 403 });
    const headers = { 'Access-Control-Allow-Origin': origin, 'Access-Control-Allow-Headers': 'Content-Type', Vary: 'Origin' };
    if (request.method === 'OPTIONS') return new Response(null, { status: 204, headers });
    const reply = (data, status = 200) => Response.json(data, { status, headers });
    if (new URL(request.url).pathname !== '/messages') return reply({ error: 'not found' }, 404);
    if (request.method === 'POST') return post(request, env, reply);
    if (request.method === 'GET') return replies(request, env, reply);
    return reply({ error: 'method not allowed' }, 405);
  },
};
