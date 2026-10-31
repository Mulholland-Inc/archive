import { chats, conversations, workspaces } from './data.js';

export const app = $state({
  workspace: workspaces[0].id,
  collapsed: typeof localStorage !== 'undefined' && localStorage.getItem('archive.sidebar') === 'collapsed',
  drawer: false,
  palette: false,
  chats: chats.map((c) => ({ ...c })),
  conversations: structuredClone(conversations),
  model: 'pro',
  draft: '',
  pending: null
});

export function toggleSidebar() {
  if (typeof matchMedia !== 'undefined' && matchMedia('(max-width: 56rem)').matches) {
    app.drawer = !app.drawer;
    return;
  }
  app.collapsed = !app.collapsed;
  localStorage.setItem('archive.sidebar', app.collapsed ? 'collapsed' : 'open');
}

export function currentWorkspace() {
  return workspaces.find((w) => w.id === app.workspace) ?? workspaces[0];
}

export function newChat(title) {
  const id = `n${Date.now().toString(36)}`;
  app.chats.unshift({ id, title, group: 'Today', updated: 'Just now' });
  app.conversations[id] = [];
  return id;
}

export function toggleStar(id) {
  const chat = app.chats.find((c) => c.id === id);
  if (chat) chat.starred = !chat.starred;
}

export function removeChat(id) {
  const i = app.chats.findIndex((c) => c.id === id);
  if (i >= 0) return app.chats.splice(i, 1)[0];
}

export function restoreChat(chat) {
  app.chats.unshift(chat);
}
