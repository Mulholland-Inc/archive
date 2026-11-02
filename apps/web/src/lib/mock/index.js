// MOCK: everything the app shows that the API cannot back yet. The rest of the app reads real data (see
// ../live.js and ../app.svelte.js); each piece here can be swapped for an API call on its own.
//
// Still mocked, and what would replace it:
// - Connectors (Settings > Connectors, the composer's "Add from a connector"): per-person app connections.
//   Needs connector endpoints (ADR 0002): list, connect (OAuth), disconnect, sync.
// - Integrations (Archive settings > Integrations: SSO, SCIM, workspace apps). Needs per-workspace
//   integration endpoints.
// - Sharing one file, folder, table or workflow with people (Share dialog, Access > Shares and guests).
//   Needs a sharing API; today access is by role and access group only.
// - Pinned chats: kept on this device. Needs a pinned flag on /api/chats.
// - Profile (name you are called, work, preferences) and notification settings: kept in this tab. Needs
//   PATCH on the signed-in user.
// - "Your API key" across every archive: tokens belong to one workspace, so the archive key is the real one.
// - Archive server settings (region, retention, default role, invites): needs a workspace settings API.
// - Onboarding checklist: kept on this device per archive.
// - Model picker, research and web search toggles, suggestions on the home screen: the API has one
//   assistant and takes only text.
// - Composer screenshots and recordings: shown as attachments but not sent.
// - Empty folders in the library: a folder is a file's access group, so a new one exists once a file is in it.
// - Workflow pause, stop and "needs input": runs are synchronous in the API.
// - The component specimen (/specimen) shows the whole design system with the made-up archives in seed.js.

import { catalog } from './connectors.js';
export { initials, models, suggestions } from './data.js';

export function mockConnectors() {
  return catalog.map((c) => ({ ...c, connected: false, account: '', lastSync: '', items: 0 }));
}

export const mockIntegrations = () => [];

export function mockKey(prefix, date) {
  const hex = () => Math.random().toString(16).slice(2, 10).padEnd(8, '0');
  return { secret: `${prefix}${hex()}${hex()}${hex()}${hex()}`, date };
}

export function mockProfile() {
  return { name: '', first: '', work: 'archivist', prefs: '', tone: 1, notify: { done: true, mentions: true, digest: true, invites: false } };
}

export const mockSettings = () => ({ defaultRole: 'viewer', invites: true, region: 'us-central1', retention: 'forever' });
