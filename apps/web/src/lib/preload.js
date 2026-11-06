
import { preloadWith } from './router.svelte.js';
import { agents, library, members, workflows, runs, schema, counts, preview, viewRows } from './app.svelte.js';
import { conversation } from './agent.svelte.js';

const chunks = {
  agent: () => import('../pages/Agent.svelte'),
  library: () => import('../pages/Library.svelte'),
  database: () => import('../pages/Database.svelte'),
  table: () => import('../pages/DatabaseTable.svelte'),
  workflows: () => import('../pages/Workflows.svelte'),
  workflow: () => import('../pages/Workflow.svelte'),
  view: () => import('../pages/View.svelte'),
  row: () => import('../pages/Row.svelte'),
  access: () => import('../pages/Access.svelte'),
  group: () => import('../pages/Group.svelte'),
  onboarding: () => import('../pages/Onboarding.svelte')
};

export function pageOf([head, a, b], onboards) {
  if (head === 'agent') return { load: chunks.agent, props: { id: a } };
  if (head === 'library') return { load: chunks.library, props: {} };
  if (head === 'view' && b) return { load: chunks.row, props: { view: a, id: b } };
  if (head === 'view' && a) return { load: chunks.view, props: { id: a } };
  if (head === 'database' && a) return { load: chunks.table, props: { id: a } };
  if (head === 'database') return { load: chunks.database, props: {} };
  if (head === 'workflows' && a) return { load: chunks.workflow, props: { id: a } };
  if (head === 'workflows') return { load: chunks.workflows, props: {} };
  if (head === 'access' && a === 'groups' && b) return { load: chunks.group, props: { id: b } };
  if (head === 'access') return { load: chunks.access, props: {} };
  if (head === 'onboarding' && onboards) return { load: chunks.onboarding, props: {} };
  return { load: chunks.agent, props: {}, home: true };
}

const pages = {
  agent: (id) => [agents.prefetch(), conversation.prefetch(id)],
  library: () => [library.prefetch(), members.prefetch()],
  database: (id) => [schema.prefetch(), counts.prefetch(), workflows.prefetch(), id && preview.prefetch(id), id && members.prefetch()],
  workflows: (id) => [workflows.prefetch(), id && runs.prefetch(id)],
  view: (id) => [schema.prefetch(), viewRows.prefetch(id)],
  access: () => members.prefetch()
};

preloadWith((path) => {
  const parts = path.split('/').filter(Boolean);
  const [page, id] = parts;
  pageOf(parts, true).load();
  pages[page]?.(id);
});
