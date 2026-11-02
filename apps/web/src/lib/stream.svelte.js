// Live data. One WebSocket per tab hears what moved in the open workspace, and the data on screen that
// depends on it is read again. Nothing polls: a read happens because something moved.
//
// The socket is api.<domain>/live?workspace=<id> (the app Worker, which hands it to the workspace's Durable
// Object), or in development `archive serve`'s own /live through vite's proxy; both send the same frames:
// {event:"ready"} once, then {event:"change", changes:[{table, id}]}, a table like "invoice" or "archive.chat"
// and, for Archive's own rows, the row's id. They name what moved, never the rows: what is read again comes
// through the API, so row-level security decides what this person sees (docs/adr/0001-architecture.md).
//
// A screen says what it shows with a resource:
//
//   export const files = resource({ tables: ['archive.file'], load: () => ... });   // in lib/
//   files.use();                                                                     // in a component
//   conversation.use(() => chatId);                                                  // with a parameter
//
// While a component that uses it is mounted, a change to one of its tables reads it again. A change to data
// nobody has on screen only marks it stale, and it is read when a screen next uses it. A hidden tab marks
// what it shows stale too, and reads it when it is shown again. A change naming a row (archive.chat <id>)
// reads only the resource whose parameter is that id.
//
// Reconnecting: the socket comes back with a backoff (1s doubling to 30s, jittered), and the backoff is only
// forgiven once a socket has held for STEADY, so a socket that opens and drops at once cannot become a
// reconnect loop. A socket that comes back may have missed changes, so what is on screen is read once more,
// at most once every SWEEP, and the rest is marked stale. That is the one read a reconnect causes: marzy's
// stream once re-read every page on every connect, and a flapping socket turned that into a storm of reads.

import { API } from './api.js';

const STEADY = 10_000;
const SWEEP = 15_000;
const PING = 30_000;

const all = new Set();
const listeners = new Set();
const refusedListeners = new Set();

export const live = $state({ status: 'off', workspace: '' });

let socket = null;
let retry = null;
let refusals = 0;
let heard = false;
let lastSweep = 0;
let sweepLater = null;
let ping = null;

const hidden = () => typeof document !== 'undefined' && document.hidden;

// A resource: data the app reads from the API, and the tables whose changes make it stale. tables is a list
// of tables, or a function of a change saying whether it touches this data. For data read by a parameter
// (a chat by its id), param says which parameter a change touches, or undefined for every one: with byId,
// a change's row id.
export function resource({ tables, load, byId = false, param: which = byId ? (c) => c.id || undefined : () => undefined }) {
  const touches = typeof tables === 'function' ? tables : (c) => tables.includes(c.table);
  const mounted = new Map();
  const fresh = new Map();
  const dirty = new Set();
  const inflight = new Map();
  const again = new Set();
  let gen = 0;

  const keyOf = (param) => (param === true ? '' : String(param));
  const stale = (key) => !fresh.has(key) || fresh.get(key) < gen || dirty.has(key);

  function refresh(param = true) {
    const key = keyOf(param);
    if (inflight.has(key)) {
      // Read again after this read only if something moved since it started.
      if (stale(key)) again.add(key);
      return inflight.get(key);
    }
    fresh.set(key, gen);
    dirty.delete(key);
    const p = Promise.resolve()
      .then(() => load(param === true ? undefined : param))
      .catch(() => {})
      .finally(() => {
        inflight.delete(key);
        if (again.delete(key)) refresh(param);
      });
    inflight.set(key, p);
    return p;
  }

  const r = {
    refresh,
    // What was just read some other way is fresh.
    seen(param = true) {
      fresh.set(keyOf(param), gen);
    },
    // use is called while a component initialises. param is a function of what it shows (a chat's id), or
    // nothing; while it returns null, false or '', the component shows nothing of this.
    use(param = () => true) {
      $effect(() => {
        const p = param();
        if (p === null || p === undefined || p === false || p === '') return;
        const key = keyOf(p);
        const held = mounted.get(key) ?? { param: p, count: 0 };
        held.count++;
        mounted.set(key, held);
        if (stale(key)) refresh(p);
        return () => {
          if (--held.count === 0) mounted.delete(key);
        };
      });
    },
    // A workspace was switched to or left: everything read so far is another workspace's, and what is on
    // screen is read for the new one.
    reset() {
      gen++;
      fresh.clear();
      dirty.clear();
      if (live.workspace) for (const [, held] of mounted) refresh(held.param);
    },
    changed(changes, sweep) {
      if (sweep) gen++;
      let touched = sweep;
      for (const c of changes) {
        if (!touches(c)) continue;
        touched = true;
        const one = which(c);
        if (one === undefined) gen++;
        else dirty.add(String(one));
      }
      if (!touched || hidden()) return;
      for (const [key, held] of mounted) if (stale(key)) refresh(held.param);
    },
    shown() {
      for (const [key, held] of mounted) if (stale(key)) refresh(held.param);
    }
  };
  all.add(r);
  return r;
}

// Hear that the socket was refused or dropped for leaving the workspace. Returns the way to stop.
export function onRefused(fn) {
  refusedListeners.add(fn);
  return () => refusedListeners.delete(fn);
}

// Hear every change frame, for what is not a resource (a cached file's bytes). Returns the way to stop.
export function onChange(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

function changed(changes, sweep = false) {
  for (const fn of listeners) fn(changes, sweep);
  for (const r of all) r.changed(changes, sweep);
}

function sweep() {
  clearTimeout(sweepLater);
  const wait = lastSweep + SWEEP - Date.now();
  if (wait > 0) {
    sweepLater = setTimeout(sweep, wait);
    return;
  }
  lastSweep = Date.now();
  changed([], true);
}

function url(workspace) {
  const base = (API || location.origin).replace(/^http/, 'ws');
  return `${base}/live?workspace=${encodeURIComponent(workspace)}`;
}

function shut() {
  clearTimeout(retry);
  clearInterval(ping);
  retry = ping = null;
  if (socket) {
    const s = socket;
    socket = null;
    try {
      s.close(1000);
    } catch {}
  }
}

function backoff() {
  if (retry || !live.workspace) return;
  refusals++;
  live.status = 'retrying';
  const wait = Math.min(30_000, 1000 * 2 ** (refusals - 1)) * (0.7 + Math.random() * 0.6);
  retry = setTimeout(dial, wait);
}

function dial() {
  retry = null;
  if (!live.workspace || socket || typeof WebSocket === 'undefined') return;
  live.status = 'connecting';
  const s = new WebSocket(url(live.workspace));
  socket = s;
  let steady;
  let waiting = false;
  let ready = false;
  s.onopen = () => {
    steady = setTimeout(() => (refusals = 0), STEADY);
    ping = setInterval(() => {
      if (waiting) return s.close(4000, 'no pong');
      waiting = true;
      try {
        s.send('ping');
      } catch {}
    }, PING);
  };
  s.onmessage = (e) => {
    if (e.data === 'pong') {
      waiting = false;
      return;
    }
    let frame = {};
    try {
      frame = JSON.parse(e.data);
    } catch {
      return;
    }
    if (frame.event === 'ready') {
      ready = true;
      live.status = 'open';
      // The first socket missed nothing: the app read everything as it opened the workspace.
      if (heard) sweep();
      heard = true;
    } else if (frame.event === 'change' && Array.isArray(frame.changes)) {
      changed(frame.changes);
    }
  };
  s.onclose = (e) => {
    clearTimeout(steady);
    clearInterval(ping);
    ping = null;
    if (socket !== s) return;
    socket = null;
    // Dropped for leaving the workspace, or refused before it was ready: this person may no longer be in
    // it, or it is gone. Whoever listens (the account) checks, once per try, which the backoff spaces out.
    if (e.code === 4003 || !ready) for (const fn of refusedListeners) fn();
    backoff();
  };
}

// Listen to a workspace, or to none with ''. Switching closes the old socket at once.
export function listen(workspace = '') {
  if (workspace === live.workspace && (socket || retry)) return;
  shut();
  live.workspace = workspace;
  heard = false;
  refusals = 0;
  for (const r of all) r.reset();
  live.status = workspace ? 'connecting' : 'off';
  if (workspace) dial();
}

if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (hidden()) return;
    for (const r of all) r.shown();
  });
  addEventListener('online', () => {
    if (retry) {
      clearTimeout(retry);
      dial();
    }
  });
}
