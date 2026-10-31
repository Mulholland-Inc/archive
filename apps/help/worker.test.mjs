import assert from 'node:assert/strict';
import { beforeEach, test } from 'node:test';
import worker from './worker.js';

const env = { ORIGINS: 'https://app.example', HELP_SECRET: 'test-secret', DISCORD_WEBHOOK: 'https://discord.test/hook', DISCORD_API: 'https://discord.test/api', DISCORD_BOT_TOKEN: 'bot-token' };
const call = (path, init = {}) => worker.fetch(new Request(`https://help.test${path}`, { ...init, headers: { Origin: 'https://app.example', ...init.headers } }), env);
const send = (body) => call('/messages', { method: 'POST', body: JSON.stringify(body) });
let sent, listed, discord;

beforeEach(() => {
  sent = [];
  listed = [];
  discord = 200;
  globalThis.fetch = async (url, init) => {
    if (init?.method === 'POST') {
      sent.push({ url: String(url), body: JSON.parse(init.body) });
      return Response.json({ id: '900', channel_id: '700' }, { status: discord });
    }
    assert.equal(init.headers.Authorization, 'Bot bot-token');
    return Response.json(listed.filter((m) => !String(url).includes('after=') || BigInt(m.id) > BigInt(String(url).match(/after=(\d+)/)[1])), { status: discord });
  };
});

test('a first message opens a post that says who sent it', async () => {
  const res = await send({ text: 'How do I store albums?', name: 'Ada', email: 'ada@example.com', archive: 'Quist Family Archive' });
  assert.equal(res.status, 200);
  assert.match(sent[0].body.content, /^\*\*Ada · ada@example\.com · Quist Family Archive\*\*\nHow do I store albums\?$/);
  assert.equal(sent[0].body.thread_name, 'How do I store albums?');
  assert.match((await res.json()).ticket, /^700\./);
});

test('nobody can ping the server through a message', async () => {
  await send({ text: '@everyone look' });
  assert.deepEqual(sent[0].body.allowed_mentions, { parse: [] });
});

test('a later message joins the same post', async () => {
  const { ticket } = await (await send({ text: 'first' })).json();
  await send({ text: 'second', ticket });
  assert.match(sent[1].url, /thread_id=700/);
  assert.equal(sent[1].body.thread_name, undefined);
});

test('replies come back in order, without the visitor’s own messages', async () => {
  const { ticket } = await (await send({ text: 'first' })).json();
  listed = [
    { id: '903', content: 'Second reply', author: { username: 'sam' } },
    { id: '902', content: 'First reply', author: { global_name: 'Marijn' } },
    { id: '901', content: 'From the webhook', webhook_id: '1', author: { bot: true } },
  ];
  const res = await call(`/messages?ticket=${encodeURIComponent(ticket)}`);
  assert.deepEqual(await res.json(), [{ id: '902', name: 'Marijn', text: 'First reply' }, { id: '903', name: 'sam', text: 'Second reply' }]);
  const later = await call(`/messages?ticket=${encodeURIComponent(ticket)}&after=902`);
  assert.deepEqual(await later.json(), [{ id: '903', name: 'sam', text: 'Second reply' }]);
});

test('a forged ticket reads and writes nothing', async () => {
  assert.equal((await call('/messages?ticket=700.forged')).status, 403);
  assert.equal((await call('/messages?ticket=700')).status, 403);
  assert.equal((await send({ text: 'hi', ticket: '701.forged' })).status, 403);
  assert.equal(sent.length, 0);
});

test('another site is refused', async () => {
  assert.equal((await call('/messages?ticket=x', { headers: { Origin: 'https://evil.example' } })).status, 403);
  assert.equal((await call('/messages', { method: 'POST', body: '{}', headers: { Origin: '' } })).status, 403);
});

test('an empty or huge message is refused', async () => {
  assert.equal((await send({ text: '   ' })).status, 400);
  assert.equal((await send({ text: 'x'.repeat(2001) })).status, 400);
  assert.equal(sent.length, 0);
});

test('a Discord failure says so', async () => {
  discord = 500;
  assert.equal((await send({ text: 'hi' })).status, 502);
});
