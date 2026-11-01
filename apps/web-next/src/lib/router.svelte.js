function parse() {
  const hash = location.hash.replace(/^#/, '') || '/';
  const parts = hash.split('/').filter(Boolean);
  return { path: hash, parts };
}

export const route = $state(parse());

if (typeof window !== 'undefined') {
  window.addEventListener('hashchange', () => {
    const next = parse();
    route.path = next.path;
    route.parts = next.parts;
  });
}

export function go(path) {
  location.hash = path;
}

export function replace(path) {
  history.replaceState(history.state, '', `#${path}`);
  const next = parse();
  route.path = next.path;
  route.parts = next.parts;
}

export function href(path) {
  return `#${path}`;
}

export function active(prefix) {
  if (prefix === '/') return route.path === '/';
  return route.path === prefix || route.path.startsWith(prefix + '/');
}
