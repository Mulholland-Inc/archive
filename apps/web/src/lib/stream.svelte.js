
import { API, api } from './api.js';

const STEADY = 10_000;
const SWEEP = 15_000;
const PING = 30_000;

const all = new Set();
const listeners = new Set();
const refusedListeners = new Set();

export const live = $state({ status: 'off', workspace: '' });

let socket = null;
let ticket = null;
let dialing = false;
let retry = null;
let refusals = 0;
let heard = false;
let lastSweep = 0;
let sweepLater = null;
let ping = null;

const hidden = () => typeof document !== 'undefined' && document.hidden;

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
    prefetch(param = true) {
      if (param === null || param === undefined || param === false || param === '') return;
      if (stale(keyOf(param))) refresh(param);
    },
    seen(param = true) {
      fresh.set(keyOf(param), gen);
    },
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

export function onRefused(fn) {
  refusedListeners.add(fn);
  return () => refusedListeners.delete(fn);
}

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

function url(t) {
  const base = (API || location.origin).replace(/^http/, 'ws');
  return `${base}/live?ticket=${encodeURIComponent(t)}`;
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

const refused = () => {
  for (const fn of refusedListeners) fn();
};

const unexpired = (t) => {
  if (!t) return false;
  try {
    return JSON.parse(atob(t.split('.')[0].replaceAll('-', '+').replaceAll('_', '/'))).exp > Date.now() / 1000;
  } catch {
    return false;
  }
};

async function dial() {
  retry = null;
  if (!live.workspace || socket || dialing || typeof WebSocket === 'undefined') return;
  live.status = 'connecting';
  const workspace = live.workspace;
  dialing = true;
  let got = null;
  let reused = false;
  let failed = false;
  try {
    got = await ticket;
    reused = unexpired(got);
    if (!reused) got = (await api('/api/boot?only=live', { workspace })).live;
  } catch {
    failed = true;
  }
  dialing = false;
  if (workspace !== live.workspace) return dial();
  if (socket) return;
  if (failed) {
    ticket = null;
    refused();
    return backoff();
  }
  ticket = got;
  if (!ticket) {
    live.status = 'off';
    return;
  }
  const s = new WebSocket(url(ticket));
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
    if (e.code === 4003 || !ready) ticket = null;
    if (e.code === 4003 || (!ready && !reused)) refused();
    if (!ready && reused) return dial();
    backoff();
  };
}

export function listen(workspace = '', given = null) {
  if (workspace === live.workspace && (socket || retry || dialing)) return;
  shut();
  live.workspace = workspace;
  ticket = given;
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
