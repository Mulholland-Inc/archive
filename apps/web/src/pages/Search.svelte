<script lang="ts">
  import { Page, PageHeader, Stack, ResultRow, Highlight, EmptyState, Notice, Skeleton, Text } from '../lib';
  import { api } from '../lib/api';
  import { session } from '../lib/session.svelte';
  import { route } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { humanize, recordTitle, formatDate, typeIcon } from '../lib/spec';
  import type { SearchHit } from '../lib/types';

  setCrumbs([{ label: 'Search' }]);

  const q = $derived(new URLSearchParams(route.search).get('q') ?? '');
  const type = $derived(new URLSearchParams(route.search).get('type') ?? '');
  let hits = $state<SearchHit[] | null>(null);
  let loading = $state(false);
  let error = $state('');
  let timer: ReturnType<typeof setTimeout>;
  let seq = 0;

  $effect(() => {
    const term = q.trim();
    const t = type;
    const space = session.space;
    clearTimeout(timer);
    timer = setTimeout(() => {
      if (!term) {
        hits = null;
        return;
      }
      const mine = ++seq;
      loading = true;
      error = '';
      api
        .search(term, { type: t || undefined, space: space || undefined, limit: 50 })
        .then((h) => {
          if (mine === seq) hits = h ?? [];
        })
        .catch((e) => {
          if (mine === seq) error = e.message;
        })
        .finally(() => {
          if (mine === seq) loading = false;
        });
    }, 220);
  });

  function hitTitle(h: SearchHit) {
    if (h.kind === 'raw') return h.source ? `Unmapped item from ${h.source}` : 'Unmapped item';
    if (h.type === 'file') return String(h.data?.name ?? 'File');
    return h.type ? recordTitle(session.ontology, { id: h.id, type: h.type, data: h.data ?? {}, spec_version: 0, created_at: h.date, updated_at: h.date, space: h.space }) : 'Record';
  }

  function hitHref(h: SearchHit) {
    if (h.kind === 'raw') return undefined;
    if (h.type === 'file') return `/library/${h.id}`;
    return `/records/${h.type}/${h.id}`;
  }

  function hitIcon(h: SearchHit) {
    if (h.kind === 'raw') return 'inbox';
    if (h.type === 'file') return 'document';
    return h.type ? typeIcon(session.ontology, h.type) : 'table';
  }
</script>

<Page width="narrow">
  <PageHeader title="Search" description="Find records, files and synced items by their words, and by meaning when semantic search is on." />
  {#if error}
    <Notice tone="danger">{error}</Notice>
  {:else if loading && !hits}
    <Skeleton rows={5} />
  {:else if hits && !hits.length}
    <EmptyState icon="search" title={`Nothing found for “${q.trim()}”`} description="Try fewer words, another spelling, or search everything instead of one type." />
  {:else if hits}
    <Stack gap="0">
      {#each hits as h (h.kind + h.id)}
        <ResultRow href={hitHref(h)} icon={hitIcon(h)} title={hitTitle(h)} meta={[h.type && h.type !== 'file' ? humanize(h.type) : '', h.date ? formatDate(h.date) : ''].filter(Boolean).join(', ')}>
          {#if h.snippet}<Highlight text={h.snippet} />{/if}
        </ResultRow>
      {/each}
    </Stack>
  {:else}
    <Text variant="muted">Type in the search bar at the top, or press ⌘K from anywhere.</Text>
  {/if}
</Page>
