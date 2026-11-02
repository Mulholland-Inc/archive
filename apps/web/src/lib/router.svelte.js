// The app's routes are plain paths, like /chat/<id>: the host serves index.html for each (wrangler.jsonc's
// single-page-application fallback, and Vite's in development).

function parse() {
  const path = location.pathname || '/';
  return { path, parts: path.split('/').filter(Boolean) };
}

export const route = $state(parse());

function sync() {
  const next = parse();
  route.path = next.path;
  route.parts = next.parts;
}

if (typeof window !== 'undefined') {
  window.addEventListener('popstate', sync);
  // A click on a link to one of the app's own paths moves there without loading the page again; one with a
  // modifier, a middle click or a target opens it as the browser would.
  document.addEventListener('click', (event) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const a = event.target.closest?.('a[href]');
    if (!a || (a.target && a.target !== '_self') || a.hasAttribute('download')) return;
    const url = new URL(a.href, location.href);
    if (!url.protocol.startsWith('http') || url.origin !== location.origin || /^\/(api|auth|mcp|files|hooks)(\/|$)/.test(url.pathname)) return;
    event.preventDefault();
    if (url.pathname + url.search + url.hash !== location.pathname + location.search + location.hash) go(url.pathname + url.search + url.hash);
  });
}

export function go(path) {
  history.pushState(null, '', path);
  sync();
}

export function replace(path) {
  history.replaceState(history.state, '', path);
  sync();
}

export function href(path) {
  return path;
}

export function active(prefix) {
  if (prefix === '/') return route.path === '/';
  return route.path === prefix || route.path.startsWith(prefix + '/');
}
