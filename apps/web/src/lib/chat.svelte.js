// Talking to the assistant: POST /api/chat streams one JSON event per line. The answer is built into the
// chat's messages as it arrives, outside any component, so it keeps going while you move around the app.

import { api, chatStream, Unauthorized } from './api.js';
import { app, ws, chatById } from './app.svelte.js';
import { markdown, step, toMessages, toChat } from './live.js';
import { replace, route } from './router.svelte.js';
import { resource } from './stream.svelte.js';

const controllers = new Map();
let seq = 0;
const mid = () => `l${++seq}`;

// A chat's messages, read when its page opens and again when it moves: a message sent from another tab, or the
// assistant answering there. While this tab streams an answer into it, the stream is what it shows; once
// that answer is saved it is the same messages, so they are kept as they were built.
export const conversation = resource({
  tables: ['archive.chat'],
  byId: true,
  load: async (id) => {
    const chat = chatById(id);
    const w = ws();
    if (!chat || chat.draft || controllers.has(id)) return;
    try {
      const c = await api(`/api/chats/${id}`, { workspace: w.id });
      chat.title = c.title || chat.title;
      chat.error = '';
      if (controllers.has(id)) return;
      // What this tab just streamed is what was saved: keep it as it was built, with how long it took.
      const saved = (c.messages ?? []).length;
      if (chat.keep) chat.keep = false;
      else if (chat.saved !== saved || !w.conversations[id]) w.conversations[id] = toMessages(c.messages ?? []);
      chat.saved = saved;
      chat.loaded = true;
    } catch (e) {
      if (e instanceof Unauthorized) app.status = 'signedOut';
      else if (!w.conversations[id]?.length) chat.error = e.message;
    }
  }
});

function adopt(w, chat, real) {
  const old = chat.id;
  w.conversations[real.id] = w.conversations[old];
  delete w.conversations[old];
  Object.assign(chat, toChat(real, chat.pinned), { draft: false, loaded: true, streaming: true });
  controllers.set(real.id, controllers.get(old));
  controllers.delete(old);
  if (route.path === `/chat/${old}`) replace(`/chat/${real.id}`);
}

// Files from the library go along as a line naming each one, so the assistant can open them by id.
function withFiles(text, attachments = []) {
  const files = attachments.filter((a) => a.real);
  if (!files.length) return text;
  return `${text}\n\nAttached from the library:\n${files.map((a) => `- ${a.name} (file ${a.id})`).join('\n')}`.trim();
}

export async function sendMessage(id, { text = '', attachments = [] }) {
  const w = ws();
  const chat = chatById(id);
  if (!chat || chat.streaming) return;
  const list = (w.conversations[chat.id] ??= []);
  list.push({ id: mid(), role: 'user', text, attachments });
  const reply = { id: mid(), role: 'assistant', steps: [], blocks: [], text: '' };
  list.push(reply);
  const live = list.at(-1);
  const ctrl = new AbortController();
  controllers.set(chat.id, ctrl);
  chat.streaming = true;
  chat.updated = 'Just now';
  const started = Date.now();
  const calls = {};
  let segment = '';
  const render = () => (live.blocks = markdown(live.text));
  try {
    const body = { text: withFiles(text, attachments) };
    if (!chat.draft) body.chat = chat.id;
    for await (const ev of chatStream(body, ctrl.signal)) {
      if (ev.type === 'chat') {
        if (chat.draft) adopt(w, chat, ev.chat);
        else if (ev.chat?.title) chat.title = ev.chat.title;
      } else if (ev.type === 'text') {
        if (!segment && live.text) live.text += '\n\n';
        segment += ev.text;
        live.text += ev.text;
        render();
      } else if (ev.type === 'thinking') {
        segment = '';
      } else if (ev.type === 'tool') {
        segment = '';
        calls[ev.id] = { id: ev.id, description: '', code: '', state: 'writing', logs: [], error: '' };
        live.steps.push(step(calls[ev.id]));
      } else if (['code', 'run', 'result'].includes(ev.type) && calls[ev.id]) {
        const c = calls[ev.id];
        if (ev.code !== undefined) c.code = ev.code;
        if (ev.description) c.description = ev.description;
        if (ev.type === 'run') c.state = 'running';
        if (ev.type === 'result') Object.assign(c, { state: 'done', logs: ev.logs ?? [], error: ev.error ?? '' });
        const i = live.steps.findIndex((s) => s.id === ev.id);
        if (i >= 0) live.steps[i] = step(c);
      } else if (ev.type === 'error') {
        live.text += `${live.text ? '\n\n' : ''}*${ev.error}*`;
        render();
      }
    }
  } catch (e) {
    if (e instanceof Unauthorized) app.status = 'signedOut';
    live.text += `${live.text ? '\n\n' : ''}*${e.name === 'AbortError' ? 'Stopped.' : e.message}*`;
    render();
  }
  for (const s of live.steps) if (s.status === 'running') s.status = 'failed';
  live.worked = `${Math.max(1, Math.round((Date.now() - started) / 1000))}s`;
  controllers.delete(chat.id);
  chat.keep = true;
  chat.streaming = false;
  if (!live.blocks.length && !live.steps.length) live.blocks = [{ type: 'p', text: '*No answer.*' }];
}

export function stopChat(id) {
  controllers.get(id)?.abort();
}
