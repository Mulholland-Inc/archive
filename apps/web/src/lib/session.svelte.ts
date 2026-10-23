import { api, setUnauthorizedHandler } from './api';
import type { Actor, Ontology, Space, View } from './types';
import { navigate, route } from './router.svelte';

type State = {
  status: 'loading' | 'signedout' | 'ready' | 'offline';
  actor: Actor | null;
  spaces: Space[];
  home: string;
  ontology: Ontology | null;
  views: View[];
  space: string;
  pending: number;
};

export const session = $state<State>({
  status: 'loading',
  actor: null,
  spaces: [],
  home: '',
  ontology: null,
  views: [],
  space: localStorage.getItem('archive.space') ?? '',
  pending: 0
});

const publicPaths = ['/signin', '/setup'];

setUnauthorizedHandler(() => {
  if (session.status === 'ready') {
    session.status = 'signedout';
    if (!publicPaths.includes(route.path)) navigate(`/signin?next=${encodeURIComponent(route.path + route.search)}`, { replace: true });
  }
});

export async function load() {
  try {
    const me = await api.me();
    const [ontology, views] = await Promise.all([api.ontology(), api.views()]);
    session.actor = me.actor;
    session.spaces = me.spaces ?? [];
    session.home = me.home;
    session.ontology = ontology;
    session.views = views ?? [];
    if (session.space && !session.spaces.some((s) => s.name === session.space)) setSpace('');
    session.status = 'ready';
  } catch (e) {
    const status = (e as { status?: number }).status;
    session.status = status === 401 ? 'signedout' : 'offline';
  }
}

export async function reloadOntology() {
  session.ontology = await api.ontology();
}

export async function reloadViews() {
  session.views = (await api.views()) ?? [];
}

export async function reloadPending() {
  session.pending = (await api.reviewCounts(session.space || undefined).catch(() => null))?.review ?? 0;
}

export async function reloadSpaces() {
  session.spaces = (await api.spaces()) ?? [];
}

export function setSpace(name: string) {
  session.space = name;
  if (name) localStorage.setItem('archive.space', name);
  else localStorage.removeItem('archive.space');
}

export function spaceName(id: string) {
  return session.spaces.find((s) => s.id === id)?.name ?? '';
}

export function spaceLabel(idOrName: string) {
  const s = session.spaces.find((x) => x.id === idOrName || x.name === idOrName);
  if (!s) return '';
  if (s.id === session.home) return 'Private';
  const t = s.name.replace(/[_-]+/g, ' ').trim();
  return t.charAt(0).toUpperCase() + t.slice(1);
}

export function canWrite(spaceId?: string) {
  const levels = ['editor', 'admin', 'owner'];
  if (!spaceId) return session.spaces.some((s) => levels.includes(s.level));
  return levels.includes(session.spaces.find((s) => s.id === spaceId)?.level ?? '');
}

export function isAdmin() {
  return session.spaces.some((s) => s.level === 'admin' || s.level === 'owner');
}

export async function signOut() {
  try {
    await api.signout();
  } finally {
    session.status = 'signedout';
    session.actor = null;
    navigate('/signin', { replace: true });
  }
}
