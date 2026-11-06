
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
  document.addEventListener('click', (event) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const a = event.target.closest?.('a[href]');
    if (!a || (a.target && a.target !== '_self') || a.hasAttribute('download')) return;
    const url = new URL(a.href, location.href);
    if (!url.protocol.startsWith('http') || url.origin !== location.origin || /^\/(api|auth|live|mcp|files|hooks)(\/|$)/.test(url.pathname)) return;
    event.preventDefault();
    if (url.pathname + url.search + url.hash !== location.pathname + location.search + location.hash) go(url.pathname + url.search + url.hash);
  });
}

let preloader = null;
let resting = null;

export function preloadWith(fn) {
  preloader = fn;
}

export function preload(path) {
  if (preloader && path) preloader(new URL(path, location.href).pathname);
}

function own(event) {
  const a = event.target.closest?.('a[href]');
  if (!a || a.hasAttribute('download')) return null;
  const url = new URL(a.href, location.href);
  if (url.origin !== location.origin || /^\/(api|auth|live|mcp|files|hooks)(\/|$)/.test(url.pathname)) return null;
  return url.pathname;
}

if (typeof window !== 'undefined') {
  document.addEventListener('pointerover', (event) => {
    const path = own(event);
    clearTimeout(resting);
    if (path) resting = setTimeout(() => preload(path), 60);
  });
  document.addEventListener('pointerout', () => clearTimeout(resting));
  for (const type of ['touchstart', 'focusin']) document.addEventListener(type, (event) => preload(own(event)), { passive: true });
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
