// Front end only for now: how each archive looks, what is locked and who its custodian is live in this browser.
export const TYPES = {
  personal: { label: 'Personal', icons: ['robot', 'safe', 'bookcase', 'pinecone'] },
  family: { label: 'Family', icons: ['robot', 'cottage', 'townhouse', 'farmhouse', 'oak', 'pine', 'baobab', 'pinecone'] },
  business: { label: 'Business', icons: ['robot', 'skyscraper', 'capitol', 'ledger', 'cabinet', 'bank', 'warehouse', 'factory'] },
};
// The colours of a national archive, as old metal and deep lacquer: maroon, gold, jungle, navy, steel, bronze and gunmetal.
export const TINTS = ['#5e1724', '#b8963e', '#1f4a38', '#22407a', '#7f8a96', '#8a6a3d', '#2e333b'];
export const KIND_TINTS = { business: '#22407a', family: '#5e1724', personal: '#b8963e' };
const RETIRED = { '#e5484d': '#5e1724', '#f5a524': '#b8963e', '#30a46c': '#1f4a38', '#0091ff': '#22407a', '#8e4ec6': '#7f8a96', '#a0703c': '#8a6a3d', '#8a2433': '#5e1724', '#2f5d4e': '#1f4a38', '#5d7690': '#7f8a96', '#b5a27a': '#8a6a3d', '#20252d': '#2e333b' };

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
  const type = looks[id]?.type ?? 'personal';
  const l = { type, icon: 'robot', tint: KIND_TINTS[type], shared: false, locked: [], shares: {}, groups: {}, custodian: null, away: [], ...looks[id] };
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
  if (!looks[id]?.type) setLook(id, { type, icon: TYPES[type].icons[1], tint: KIND_TINTS[type] });
}

export function setLook(id, patch) {
  looks[id] = { ...looks[id], ...patch };
  persist();
}
