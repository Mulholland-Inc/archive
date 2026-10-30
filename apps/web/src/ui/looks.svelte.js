// Front end only for now: how each archive looks, what is locked and who its custodian is live in this browser.
export const TYPES = {
  personal: { label: 'Personal', icons: ['robot', 'safe', 'bookcase'] },
  family: { label: 'Family', icons: ['robot', 'cottage', 'townhouse', 'farmhouse'] },
  business: { label: 'Business', icons: ['robot', 'warehouse', 'factory', 'tower'] },
};
export const TINTS = ['#e5484d', '#f5a524', '#30a46c', '#0091ff', '#8e4ec6', '#a0703c', '#2a2f38'];

function load() {
  try {
    return JSON.parse(localStorage.looks ?? '{}');
  } catch {
    return {};
  }
}

const looks = $state(load());

function floating() {
  try {
    return localStorage.floating !== 'off';
  } catch {
    return true;
  }
}

export const ui = $state({ floating: floating() });

export function setFloating(on) {
  ui.floating = on;
  try {
    localStorage.floating = on ? 'on' : 'off';
  } catch {}
}

function persist() {
  try {
    localStorage.looks = JSON.stringify(looks);
  } catch {}
}

export function look(id = '') {
  const tint = TINTS[[...id].reduce((n, c) => n + c.charCodeAt(0), 0) % TINTS.length];
  return { type: 'personal', icon: 'robot', tint, shared: false, locked: [], shares: {}, custodian: null, ...looks[id] };
}

// What the sidebar and chat call an archive: you type Brenner, and the type finishes it.
export function shortName(name, type) {
  if (type !== 'family') return name;
  return /\bfamily\b/i.test(name) ? name.replace(/\bfamily\b/i, 'Family') : `${name} Family`;
}

export function fullName(name, type) {
  if (/\barchive$/i.test(name)) return name;
  if (type === 'personal' && name.toLowerCase() !== 'personal') return `${name}'s Archive`;
  return `${shortName(name, type)} Archive`;
}

// Spaces other archives have shared into this one.
export function incoming(id) {
  return Object.keys(looks).flatMap((from) => Object.entries(looks[from].shares ?? {}).filter(([, to]) => to.includes(id)).map(([space]) => ({ from, space })));
}

export function setLook(id, patch) {
  looks[id] = { ...looks[id], ...patch };
  persist();
}
