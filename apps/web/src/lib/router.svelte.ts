export const route = $state({ path: location.pathname, search: location.search });

export function query(): URLSearchParams {
  return new URLSearchParams(route.search);
}

export function navigate(to: string, { replace = false } = {}) {
  const url = new URL(to, location.origin);
  if (url.pathname === route.path && url.search === route.search) return;
  if (replace) history.replaceState(null, '', url.pathname + url.search);
  else history.pushState(null, '', url.pathname + url.search);
  route.path = url.pathname;
  route.search = url.search;
  if (!replace) window.scrollTo(0, 0);
}

export function setQuery(params: { [key: string]: string | undefined | null }, replace = true) {
  const q = query();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === '') q.delete(k);
    else q.set(k, v);
  }
  const s = q.toString();
  navigate(route.path + (s ? `?${s}` : ''), { replace });
}

addEventListener('popstate', () => {
  route.path = location.pathname;
  route.search = location.search;
});

export function match(pattern: string, path = route.path): { [k: string]: string } | null {
  const p = pattern.split('/').filter(Boolean);
  const s = path.split('/').filter(Boolean);
  if (p.length !== s.length) return null;
  const params: { [k: string]: string } = {};
  for (let i = 0; i < p.length; i++) {
    if (p[i].startsWith(':')) params[p[i].slice(1)] = decodeURIComponent(s[i]);
    else if (p[i] !== s[i]) return null;
  }
  return params;
}

export function intercept(e: MouseEvent, href: string) {
  if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  const url = new URL(href, location.origin);
  if (url.origin !== location.origin || url.pathname.startsWith('/api/') || url.pathname.startsWith('/auth/') || url.pathname.startsWith('/files/')) return;
  e.preventDefault();
  navigate(url.pathname + url.search);
}
