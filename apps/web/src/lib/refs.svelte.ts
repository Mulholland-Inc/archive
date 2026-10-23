import { api } from './api';
import { recordTitle } from './spec';
import { session } from './session.svelte';
import type { ArchiveRecord } from './types';

const titles = $state<{ [id: string]: string }>({});
const pending = new Set<string>();

export function remember(r: ArchiveRecord) {
  titles[r.id] = recordTitle(session.ontology, r);
}

export function refTitle(type: string, id: string): string {
  if (titles[id] !== undefined) return titles[id];
  if (!pending.has(id)) {
    pending.add(id);
    api
      .get(type, id)
      .then((r) => remember(r))
      .catch(() => (titles[id] = `${type} ${id.slice(-6)}`));
  }
  return id.slice(-6);
}
