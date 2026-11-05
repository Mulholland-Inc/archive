
import { api, turnStream, Unauthorized } from './api.js';
import { app, ws, agentById, openFile, clearUnread } from './app.svelte.js';
import { route } from './router.svelte.js';
import { changed, markdown, step, toMessages } from './live.js';
import { resource } from './stream.svelte.js';

const PAGE = 20;
const controllers = new Map();
let seq = 0;
const mid = () => `l${++seq}`;

export const conversation = resource({
  tables: ['archive.turn'],
  load: async (id) => {
    const agent = agentById(id);
    const w = ws();
    if (!agent || controllers.has(id)) return;
    agent.limit ??= PAGE;
    try {
      const turns = (await api(`/api/agent/${id}/turns?limit=${agent.limit}`, { workspace: w.id })) ?? [];
      agent.error = '';
      if (controllers.has(id)) return;
      const messages = turns.flatMap((t) => t.messages);
      if (agent.keep) agent.keep = false;
      else if (agent.saved !== messages.length || !w.conversations[id]) w.conversations[id] = toMessages(messages);
      agent.saved = messages.length;
      agent.more = turns.length === agent.limit;
      agent.loaded = true;
    } catch (e) {
      if (e instanceof Unauthorized) app.status = 'signedOut';
      else if (!w.conversations[id]?.length) agent.error = e.message;
    }
  }
});

export function loadEarlier(id) {
  const agent = agentById(id);
  if (!agent?.more || agent.streaming) return null;
  agent.more = false;
  agent.limit += PAGE;
  agent.keep = false;
  return conversation.refresh(id);
}

function withFiles(text, attachments) {
  if (!attachments.length) return text;
  return `${text}\n\nAttached from the library:\n${attachments.map((a) => `- ${a.name} (file ${a.id})`).join('\n')}`.trim();
}

export async function sendMessage(id, { text = '', attachments = [] }) {
  const w = ws();
  const agent = agentById(id);
  if (!agent || agent.streaming) return;
  w.conversations[id] ??= [];
  const list = w.conversations[id];
  list.push({ id: mid(), role: 'user', text, attachments });
  list.push({ id: mid(), role: 'assistant', steps: [], blocks: [], text: '', files: [] });
  const live = list.at(-1);
  const ctrl = new AbortController();
  controllers.set(id, ctrl);
  agent.streaming = true;
  const started = Date.now();
  const calls = {};
  let segment = '';
  const render = () => (live.blocks = markdown(live.text));
  const note = (text) => {
    live.text += `${live.text ? '\n\n' : ''}*${text}*`;
    render();
  };
  let finished = false;
  const finish = () => {
    if (finished) return;
    finished = true;
    for (const s of live.steps) if (s.status === 'running') s.status = 'failed';
    live.worked = `${Math.max(1, Math.round((Date.now() - started) / 1000))}s`;
    controllers.delete(id);
    agent.keep = true;
    agent.streaming = false;
    if (app.archive === w.id) clearUnread(w);
    else w.unread = true;
    if (!live.blocks.length && !live.steps.length) live.blocks = [{ type: 'p', text: '*No answer.*' }];
    const shown = live.files.findLast((f) => f.action !== 'read') ?? live.files.at(-1);
    if (shown && !ctrl.signal.aborted && route.parts[0] === 'agent' && route.parts[1] === id && !matchMedia('(max-width: 56rem)').matches) openFile(shown.id, shown.name);
  };
  try {
    for await (const ev of turnStream({ agent: id, text: withFiles(text, attachments) }, ctrl.signal)) {
      if (ev.type === 'done') {
        finish();
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
        if (ev.type === 'result') {
          Object.assign(c, { state: 'done', logs: ev.logs ?? [], error: ev.error ?? '' });
          changed(live, ev.files);
        }
        const i = live.steps.findIndex((s) => s.id === ev.id);
        if (i >= 0) live.steps[i] = step(c);
      } else if (ev.type === 'error') {
        note(ev.error);
      }
    }
  } catch (e) {
    if (e instanceof Unauthorized) app.status = 'signedOut';
    if (!finished) note(e.name === 'AbortError' ? 'Stopped.' : e.message);
  }
  finish();
}

export async function clearAgent(id) {
  const w = ws();
  const agent = agentById(id);
  if (!agent) return;
  controllers.get(id)?.abort();
  await api(`/api/agent/${id}/turns`, { method: 'DELETE', workspace: w.id });
  w.conversations[id] = [];
  Object.assign(agent, { saved: 0, limit: PAGE, more: false, keep: false });
}

export function stopAgent(id) {
  controllers.get(id)?.abort();
}
