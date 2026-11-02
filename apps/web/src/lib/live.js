// Maps what the API returns onto the shapes the screens were designed with (see mock/ for the made-up
// versions). Each mapper takes one API object and returns one screen object, so a screen can move from
// mock to real data one piece at a time.

import { fileURL } from './api.js';

const DAY = 864e5;

export function relative(value, now = Date.now()) {
  if (!value) return '';
  const t = new Date(value).getTime();
  const s = Math.max(0, Math.round((now - t) / 1000));
  if (s < 60) return 'Just now';
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  const d = new Date(t);
  const today = new Date(now);
  today.setHours(0, 0, 0, 0);
  if (t >= today.getTime()) return s < 6 * 3600 ? `${Math.floor(s / 3600)} hr ago` : 'Today';
  if (t >= today.getTime() - DAY) return 'Yesterday';
  if (t >= today.getTime() - 6 * DAY) return d.toLocaleDateString('en-US', { weekday: 'short' });
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', ...(d.getFullYear() !== today.getFullYear() && { year: 'numeric' }) });
}

export function size(n = 0) {
  if (n < 1024) return `${n} B`;
  if (n < 1 << 20) return `${Math.max(1, Math.round(n / 1024))} KB`;
  return `${(n / (1 << 20)).toFixed(1)} MB`;
}

export function elapsed(ms = 0) {
  const s = Math.round(ms / 1000);
  if (s < 1) return `${Math.max(0, ms)}ms`;
  const m = Math.floor(s / 60);
  return m ? `${m}m ${String(s % 60).padStart(2, '0')}s` : `${s}s`;
}

// One cell of a row as text: money, names, addresses and the other composite types read as people write them.
export function show(v) {
  if (v === null || v === undefined) return '';
  if (Array.isArray(v)) return v.map(show).join(', ');
  if (typeof v === 'object') {
    if ('amount' in v && 'currency' in v) return `${v.amount} ${v.currency}`;
    if ('utc' in v) return new Date(v.utc).toLocaleString();
    if ('value' in v && 'unit' in v) return `${v.value} ${v.unit}`;
    if ('full' in v) return v.full ?? [v.given, v.family].filter(Boolean).join(' ');
    if ('lines' in v) return [...(v.lines ?? []), v.locality, v.country].filter(Boolean).join(', ');
    if ('lat' in v) return `${v.lat.toFixed(4)}, ${v.lon.toFixed(4)}`;
    return Object.values(v).filter((x) => x !== null && x !== '').map(show).join(' ');
  }
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(v)) return new Date(v).toLocaleString();
  return String(v);
}

const hash = (s = '') => [...s].reduce((n, c) => (n * 31 + c.charCodeAt(0)) >>> 0, 7);
export const tone = (s, n = 4) => (hash(s) % n) + 1;

export function fileKind(mime = '', name = '') {
  const ext = name.match(/\.([a-z0-9]+)$/i)?.[1]?.toLowerCase() ?? '';
  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('audio/') || mime.startsWith('video/')) return 'recording';
  if (mime === 'application/pdf' || ext === 'pdf') return 'pdf';
  if (/sheet|excel|csv/.test(mime) || ['csv', 'xls', 'xlsx', 'ods', 'numbers'].includes(ext)) return 'spreadsheet';
  return 'document';
}

function clock(seconds) {
  const s = Math.round(seconds);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

export function toFile(f, owner = '') {
  const ext = (f.name.match(/\.([a-z0-9]+)$/i)?.[1] ?? f.mime?.split('/')[1] ?? '').toUpperCase().slice(0, 4);
  const duration = Number(f.metadata?.duration ?? f.metadata?.seconds);
  return {
    id: f.id,
    name: f.name,
    kind: fileKind(f.mime, f.name),
    ext,
    mime: f.mime,
    bytes: f.size,
    size: size(f.size),
    duration: duration ? clock(duration) : '',
    modified: relative(f.created_at),
    created: f.created_at,
    owner,
    folder: f.access ?? 'Everyone',
    access: f.access,
    status: f.status,
    href: fileURL(f.id),
    path: `/api/files/${f.id}/content`,
    thumb: f.metadata?.thumbnail ? `/api/files/${f.id}/thumbnail` : '',
    tone: tone(f.id)
  };
}

export const toChat = (c, pinned = false) => ({ id: c.id, title: c.title || 'Untitled', updated: relative(c.updated_at ?? c.created_at), at: c.updated_at, pinned });

export function toMember(p, me) {
  const mine = me && p.email === me.email;
  return { id: p.id, user: mine ? me.user_id : undefined, name: p.name || p.email, email: p.email, role: p.role, groups: p.groups ?? [], active: mine ? 'Now' : '', status: 'active', tone: tone(p.email, 6) };
}

// Markdown, as the assistant writes it, to the blocks Prose renders.
export function markdown(src = '') {
  const blocks = [];
  const lines = src.replace(/\r/g, '').split('\n');
  let para = [];
  const flush = () => {
    if (para.length) blocks.push({ type: 'p', text: para.join(' ').trim() });
    para = [];
  };
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const fence = line.match(/^\s*```\s*([\w+-]*)/);
    if (fence) {
      flush();
      const code = [];
      for (i++; i < lines.length && !/^\s*```/.test(lines[i]); i++) code.push(lines[i]);
      blocks.push({ type: 'code', code: code.join('\n'), lang: fence[1] || 'text' });
      continue;
    }
    if (!line.trim()) {
      flush();
      continue;
    }
    const h = line.match(/^#{1,6}\s+(.*)$/);
    if (h) {
      flush();
      blocks.push({ type: 'h', text: h[1].replace(/\*\*/g, '') });
      continue;
    }
    if (/^\s*>/.test(line)) {
      flush();
      const quote = [];
      for (; i < lines.length && /^\s*>/.test(lines[i]); i++) quote.push(lines[i].replace(/^\s*>\s?/, ''));
      i--;
      blocks.push({ type: 'quote', text: quote.join(' ') });
      continue;
    }
    if (/^\s*\|.*\|\s*$/.test(line) && /^\s*\|?\s*:?-{2,}/.test(lines[i + 1] ?? '')) {
      flush();
      const cells = (l) => l.trim().replace(/^\||\|$/g, '').split('|').map((c) => c.trim());
      const head = cells(line);
      const rows = [];
      for (i += 2; i < lines.length && /^\s*\|.*\|\s*$/.test(lines[i]); i++) rows.push(cells(lines[i]));
      i--;
      blocks.push({ type: 'table', head, rows });
      continue;
    }
    const item = line.match(/^\s*([-*+]|\d+[.)])\s+(.*)$/);
    if (item) {
      flush();
      const type = /\d/.test(item[1]) ? 'ol' : 'ul';
      const items = [];
      for (; i < lines.length; i++) {
        const m = lines[i].match(/^\s*([-*+]|\d+[.)])\s+(.*)$/);
        if (m && (/\d/.test(m[1]) ? 'ol' : 'ul') === type) items.push(m[2]);
        else if (lines[i].trim() && /^\s{2,}/.test(lines[i]) && items.length) items[items.length - 1] += ` ${lines[i].trim()}`;
        else break;
      }
      i--;
      blocks.push({ type, items });
      continue;
    }
    para.push(line.trim());
  }
  flush();
  return blocks;
}

// A tool call the assistant made, as a step. Its description says what it does, or else the code's first comment.
export function step(call) {
  const said = call.description?.trim() || call.code?.match(/^\s*\/\/\s*(.+)$/m)?.[1];
  const failed = call.state === 'done' && call.error;
  const label = said || (call.state === 'writing' ? 'Writing code' : call.state === 'running' ? 'Running code' : 'Ran code');
  return {
    id: call.id,
    label,
    detail: failed ? call.error : (call.logs ?? []).slice(-1)[0] ?? '',
    status: call.state === 'done' ? (failed ? 'failed' : 'done') : 'running',
    time: '',
    code: call.code
  };
}

function outcome(b) {
  const raw = typeof b.content === 'string' ? b.content : (b.content ?? []).map((c) => c.text ?? '').join('');
  try {
    const o = JSON.parse(raw);
    return { logs: o.logs ?? [], error: o.error ?? '', state: 'done' };
  } catch {
    return { logs: [], error: b.is_error ? raw : '', state: 'done' };
  }
}

// A saved chat's messages (Anthropic's format) to the screen's messages: one user bubble per question,
// one assistant message per answer with its tool calls as steps and its text as prose.
export function toMessages(messages = []) {
  const out = [];
  const calls = {};
  let turn = null;
  let n = 0;
  const reply = () => {
    turn = { id: `m${n++}`, role: 'assistant', steps: [], blocks: [], text: '', calls: [] };
    out.push(turn);
  };
  for (const m of messages) {
    const content = typeof m.content === 'string' ? [{ type: 'text', text: m.content }] : (m.content ?? []);
    if (m.role === 'user') {
      for (const b of content) if (b.type === 'tool_result' && calls[b.tool_use_id]) Object.assign(calls[b.tool_use_id], outcome(b));
      const text = content.filter((b) => b.type === 'text').map((b) => b.text).join('\n');
      if (text) {
        out.push({ id: `m${n++}`, role: 'user', text });
        turn = null;
      }
      continue;
    }
    if (!turn) reply();
    for (const b of content) {
      if (b.type === 'text' && b.text.trim()) turn.text += (turn.text ? '\n\n' : '') + b.text;
      if (b.type === 'tool_use') turn.calls.push((calls[b.id] = { id: b.id, description: b.input?.description ?? '', code: b.input?.code ?? '', state: 'done', logs: [], error: '' }));
    }
  }
  for (const m of out) {
    if (m.role !== 'assistant') continue;
    m.steps = m.calls.map(step);
    m.blocks = markdown(m.text);
    delete m.calls;
  }
  return out;
}

// Workflows: the file name is the id; its trigger is the `on` it exports.

const days = { '*': 'Every day', '1-5': 'Weekdays', 'MON-FRI': 'Weekdays', '0,6': 'Weekends', '6,0': 'Weekends', 'SAT,SUN': 'Weekends', 0: 'Sundays', 7: 'Sundays', 1: 'Mondays', 2: 'Tuesdays', 3: 'Wednesdays', 4: 'Thursdays', 5: 'Fridays', 6: 'Saturdays', SUN: 'Sundays', MON: 'Mondays', TUE: 'Tuesdays', WED: 'Wednesdays', THU: 'Thursdays', FRI: 'Fridays', SAT: 'Saturdays' };
const named = { '@hourly': ['Every hour'], '@daily': ['Every day', '0', '0'], '@midnight': ['Every day', '0', '0'], '@weekly': ['Sundays', '0', '0'], '@monthly': ['On the 1st of every month', '0', '0'], '@yearly': ['Every January 1st', '0', '0'], '@annually': ['Every January 1st', '0', '0'] };
const units = { s: 'second', m: 'minute', h: 'hour' };
const nth = (n) => n + ({ 1: 'st', 2: 'nd', 3: 'rd' }[(n % 10) * (Math.floor(n / 10) % 10 !== 1)] ?? 'th');

function schedule(spec) {
  let zone = 'UTC';
  const tz = spec.match(/^(?:CRON_)?TZ=(\S+)\s+(.*)$/);
  if (tz) [zone, spec] = [`${tz[1].split('/').pop().replaceAll('_', ' ')} time`, tz[2]];
  const at = (h, m) => ` at ${+h}:${String(+m).padStart(2, '0')} ${zone}`;
  const every = spec.match(/^@every\s+(\d+)([smh])$/);
  if (every) return +every[1] === 1 ? `Every ${units[every[2]]}` : `Every ${every[1]} ${units[every[2]]}s`;
  if (named[spec]) return named[spec][0] + (named[spec][1] ? at(named[spec][1], named[spec][2]) : '');
  const [m, h, dom, mon, dow, extra] = spec.split(/\s+/);
  const num = (s) => /^\d+$/.test(s ?? '');
  if (extra === undefined && num(m) && num(h) && mon === '*') {
    if (dom === '*' && days[dow?.toUpperCase()]) return days[dow.toUpperCase()] + at(h, m);
    if (num(dom) && dow === '*') return `On the ${nth(+dom)} of every month` + at(h, m);
  }
  if (extra === undefined && num(m) && h === '*' && dom === '*' && mon === '*' && dow === '*') return `Every hour at :${m.padStart(2, '0')}`;
  const stepped = m?.match(/^\*\/(\d+)$/);
  if (extra === undefined && stepped && h === '*' && dom === '*' && mon === '*' && dow === '*') return `Every ${stepped[1]} minutes`;
  return 'On a schedule';
}

export function trigger(on) {
  if (!on) return { kind: 'manual', label: 'Run by hand' };
  if (on === 'webhook') return { kind: 'webhook', label: 'When its webhook is called' };
  if (on.startsWith('cron ')) return { kind: 'schedule', label: schedule(on.slice(5).trim()) };
  const t = on.replaceAll('_', ' ');
  return { kind: on === 'file' ? 'file' : 'table', table: on, label: `When ${/^[aeiou]/i.test(t) ? 'an' : 'a'} ${t} is added or changed` };
}

export const pretty = (name) => {
  const s = name.replace(/\.ts$/, '').replaceAll(/[-_]+/g, ' ');
  return s.charAt(0).toUpperCase() + s.slice(1);
};

const json = (v) => (typeof v === 'string' ? v : JSON.stringify(v, null, 2));

function ranBy(t = '') {
  if (!t || t === 'run' || t === 'start') return 'Started by hand';
  if (t === 'cron' || t.startsWith('cron')) return 'Ran on its schedule';
  if (t === 'webhook') return 'Called through its webhook';
  return `Ran for changes to ${t.replaceAll('_', ' ')}`;
}

export function toRun(r, number) {
  const logs = Array.isArray(r.logs) ? r.logs : [];
  const failed = Boolean(r.error);
  const log = [];
  if (logs.length) log.push({ id: `${r.id}-logs`, role: 'assistant', blocks: [{ type: 'h', text: 'Logs' }, { type: 'code', code: logs.map(json).join('\n'), lang: 'text' }] });
  if (r.result !== null && r.result !== undefined) log.push({ id: `${r.id}-result`, role: 'assistant', blocks: [{ type: 'h', text: 'Result' }, { type: 'code', code: json(r.result), lang: 'json' }] });
  return {
    id: r.id,
    number,
    status: failed ? 'failed' : 'succeeded',
    started: relative(r.at),
    elapsed: elapsed(r.ms),
    seconds: Math.round((r.ms ?? 0) / 1000),
    steps: [{ id: 's0', label: ranBy(r.trigger), detail: r.error ?? '', status: failed ? 'failed' : 'done', time: elapsed(r.ms) }],
    log
  };
}

export function toWorkflow(w) {
  return {
    id: w.name,
    name: pretty(w.name),
    file: w.name,
    agent: 'Workflow',
    trigger: trigger(w.on),
    on: w.on,
    url: w.url ?? '',
    next: w.next ? new Date(w.next).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }) : '',
    enabled: true,
    steps: [`run ${w.name}`],
    runs: w.last ? [toRun(w.last, 1)] : [],
    loaded: false
  };
}
