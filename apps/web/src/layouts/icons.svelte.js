import { layoutOf } from './index.js';

const loaded = $state({});
const asked = new Set();

export function viewIcon(view) {
  const name = view?.icon;
  if (name && !asked.has(name)) {
    asked.add(name);
    import('../lib/lucide.js').then((m) => m.load(name)).then((icon) => (loaded[name] = icon));
  }
  return loaded[name] ?? layoutOf(view).icon;
}
