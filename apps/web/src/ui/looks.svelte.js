// Front end only for now: how each archive looks, what is locked and who its custodian is live in this browser.
export const TYPES = {
  personal: { label: 'Personal', icons: ['robot', 'safe', 'bookcase'] },
  family: { label: 'Family', icons: ['robot', 'cottage', 'townhouse', 'farmhouse'] },
  business: { label: 'Business', icons: ['robot', 'cabinet', 'warehouse', 'factory', 'bank'] },
};
// The colours of a national archive: oxblood, gold leaf, Treasury green, federal navy, slate, parchment and ink.
export const TINTS = ['#8a2433', '#b8963e', '#2f5d4e', '#22407a', '#5d7690', '#b5a27a', '#20252d'];
const RETIRED = { '#e5484d': '#8a2433', '#f5a524': '#b8963e', '#30a46c': '#2f5d4e', '#0091ff': '#22407a', '#8e4ec6': '#5d7690', '#a0703c': '#b5a27a' };

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
  const l = { type: 'personal', icon: 'robot', tint, shared: false, locked: [], shares: {}, groups: {}, custodian: null, away: [], ...looks[id] };
  return { ...l, tint: RETIRED[l.tint] ?? l.tint };
}

// What the sidebar and chat call an archive: you type a name, and the type finishes it.
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

// Local demo only: the same starting look in every browser, under anything set here.
export async function seedLooks() {
  try {
    const seed = await (await fetch('/demo-looks.json')).json();
    for (const [id, l] of Object.entries(seed)) looks[id] = { ...l, ...looks[id] };
    persist();
  } catch {}
}

// What an archive holds decides its type until someone picks one.
export function settleType(id, type) {
  if (!looks[id]?.type) setLook(id, { type, icon: TYPES[type].icons[1] });
}

export function setLook(id, patch) {
  looks[id] = { ...looks[id], ...patch };
  persist();
}
