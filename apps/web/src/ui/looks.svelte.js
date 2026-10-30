// Front end only for now: how each archive looks, what is locked and who its custodian is live in this browser.
export const TYPES = {
  personal: { label: 'Personal', icons: ['robot', 'user', 'key-round', 'book-open', 'notebook-pen', 'gem'] },
  family: { label: 'Family', icons: ['robot', 'trees', 'house', 'heart', 'book-heart', 'camera'] },
  business: { label: 'Business', icons: ['robot', 'landmark', 'building-2', 'briefcase', 'scale', 'vault'] },
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

function persist() {
  try {
    localStorage.looks = JSON.stringify(looks);
  } catch {}
}

export function look(id = '') {
  const tint = TINTS[[...id].reduce((n, c) => n + c.charCodeAt(0), 0) % TINTS.length];
  return { type: 'personal', icon: 'robot', tint, shared: false, locked: [], custodian: null, ...looks[id] };
}

export function setLook(id, patch) {
  looks[id] = { ...looks[id], ...patch };
  persist();
}
