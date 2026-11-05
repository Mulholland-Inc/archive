// What each of the app's pages reads, so a link to it can read that before it is followed (router.svelte.js).
// A resource read this way is fresh when the page uses it, so the page shows it at once; one that moved since
// is read again as usual.

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
