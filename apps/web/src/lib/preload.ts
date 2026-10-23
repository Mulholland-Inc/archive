import { api } from './api';
import { match } from './router.svelte';
import { session } from './session.svelte';

const quiet = () => {};

export function preloadRecord(type: string, id: string) {
  api.get(type, id).catch(quiet);
  api.links(type, id).catch(quiet);
}

export function preloadList(type: string, where: { [field: string]: string } = {}) {
  api.list(type, { space: session.space || undefined, where, limit: 100 }).catch(quiet);
}

export function preload(href: string) {
  const url = new URL(href, location.origin);
  if (url.origin !== location.origin) return;
  const record = match('/records/:type/:id', url.pathname);
  if (record) {
    preloadRecord(record.type, record.id);
    api.provenance(record.type, record.id).catch(quiet);
    return;
  }
  const view = match('/views/:id', url.pathname);
  if (view) {
    api.view(view.id).then((v) => preloadList(v.type, v.where ?? {})).catch(quiet);
    return;
  }
  const list = match('/records/:type', url.pathname);
  if (list && !url.search) preloadList(list.type);
}
