
import { preloadWith } from './router.svelte.js';
import { agents, library, members, workflows, runs, schema, counts, preview, viewRows } from './app.svelte.js';
import { conversation } from './agent.svelte.js';

const pages = {
  agent: (id) => [agents.prefetch(), conversation.prefetch(id)],
  library: () => [library.prefetch(), members.prefetch()],
  database: (id) => [schema.prefetch(), counts.prefetch(), workflows.prefetch(), id && preview.prefetch(id), id && members.prefetch()],
  workflows: (id) => [workflows.prefetch(), id && runs.prefetch(id)],
  view: (id) => [schema.prefetch(), viewRows.prefetch(id)],
  access: () => members.prefetch()
};

preloadWith((path) => {
  const [page, id] = path.split('/').filter(Boolean);
  pages[page]?.(id);
});
