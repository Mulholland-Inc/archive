<script lang="ts">
  import { Page, PageHeader, Table, Highlight, EmptyState, Notice, Skeleton, Text, Icon, Sheet } from '../lib';
  import { api } from '../lib/api';
  import { session } from '../lib/session.svelte';
  import { route, navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { fieldsOf, humanize, recordTitle, titleField, formatDate, typeIcon } from '../lib/spec';
  import { preloadRecord } from '../lib/preload';
  import type { ArchiveRecord, SearchHit } from '../lib/types';
  import FieldValue from './parts/FieldValue.svelte';
  import RecordPeek from './parts/RecordPeek.svelte';

  setCrumbs([{ label: 'Search' }]);

  const q = $derived(new URLSearchParams(route.search).get('q') ?? '');
  const type = $derived(new URLSearchParams(route.search).get('type') ?? '');
  let hits = $state<SearchHit[] | null>(null);
  let loading = $state(false);
  let error = $state('');
  let peek = $state<{ type: string; id: string } | null>(null);
  let timer: ReturnType<typeof setTimeout>;
  let seq = 0;

  const o = $derived(session.ontology);

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

  const groups = $derived.by(() => {
    const out = new Map<string, SearchHit[]>();
    for (const h of hits ?? []) {
      const key = h.kind === 'raw' ? 'raw' : (h.type ?? 'record');
      out.set(key, [...(out.get(key) ?? []), h]);
    }
    return [...out].map(([key, items]) => ({ key, items }));
  });

  function asRecord(h: SearchHit): ArchiveRecord {
    return { id: h.id, type: h.type ?? '', data: h.data ?? {}, spec_version: 0, created_at: h.date, updated_at: h.date, space: h.space };
  }

  function heading(key: string) {
    if (key === 'raw') return { label: 'Unmapped items', icon: 'inbox' };
    if (key === 'file') return { label: 'Files', icon: 'document' };
    return { label: humanize(key), icon: typeIcon(o, key) };
  }

  function columns(key: string, items: SearchHit[]) {
    const match = items.some((x) => x.snippet) ? [{ id: '__match', label: 'Match', width: 'wide' as const }] : [];
    if (key === 'raw' || key === 'file') return [{ id: '__title', label: key === 'file' ? 'Name' : 'Source' }, ...match, { id: '__date', label: 'Added', width: 'narrow' as const }];
    const tf = titleField(o, key);
    const extra = fieldsOf(o, key)
      .filter(([n, f]) => n !== tf && !f.list)
      .slice(0, 3)
      .map(([n]) => ({ id: n, label: humanize(n) }));
    return [{ id: '__title', label: tf ? humanize(tf) : 'Name' }, ...extra, ...match];
  }

  function open(h: SearchHit) {
    if (h.kind === 'raw') return;
    if (h.type === 'file') navigate(`/library/${h.id}`);
    else if (h.type) peek = { type: h.type, id: h.id };
  }
</script>

<Page>
  <PageHeader title="Search" />
  {#if error}
    <Notice tone="danger">{error}</Notice>
  {:else if loading && !hits}
    <Skeleton rows={5} />
  {:else if hits && !hits.length}
    <EmptyState icon="search" title={`Nothing found for “${q.trim()}”`} description="Try fewer words, another spelling, or search everything instead of one type." />
  {:else if hits}
    {#each groups as g (g.key)}
      {@const h = heading(g.key)}
      <section class="group">
        <h2 class="title"><Icon name={h.icon} size="sm" />{h.label}<span class="count">{g.items.length}</span></h2>
        <Table
          columns={columns(g.key, g.items)}
          rows={g.items}
          key={(x) => x.kind + x.id}
          onrowclick={g.key === 'raw' ? undefined : open}
          onrowhover={(x) => x.kind !== 'raw' && x.type && x.type !== 'file' && preloadRecord(x.type, x.id)}
        >
          {#snippet cell(x, c)}
            {#if c === '__title'}
              <Text variant="strong">{x.kind === 'raw' ? (x.source ?? 'Unknown source') : x.type === 'file' ? String(x.data?.name ?? 'File') : recordTitle(o, asRecord(x))}</Text>
            {:else if c === '__match'}
              {#if x.snippet}<Highlight text={x.snippet} />{/if}
            {:else if c === '__date'}
              <Text variant="muted">{x.date ? formatDate(x.date) : ''}</Text>
            {:else}
              {@const f = fieldsOf(o, g.key).find(([n]) => n === c)?.[1]}
              {#if f && x.data?.[c] !== undefined && x.data?.[c] !== null}<FieldValue field={f} value={x.data[c]} />{/if}
            {/if}
          {/snippet}
        </Table>
      </section>
    {/each}
  {:else}
    <Text variant="muted">Type in the search bar at the top, or press ⌘K from anywhere.</Text>
  {/if}
</Page>

<Sheet open={!!peek} label="Record" onclose={() => (peek = null)}>
  {#if peek}
    {#key `${peek.type}/${peek.id}`}
      <RecordPeek type={peek.type} id={peek.id} onopen={(t, i) => (peek = { type: t, id: i })} onclose={() => (peek = null)} />
    {/key}
  {/if}
</Sheet>

<style>
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-base);
    font-weight: var(--font-semibold);
  }
  .title :global(svg) {
    color: var(--muted-foreground);
  }
  .count {
    color: var(--muted-foreground);
    font-weight: var(--font-medium);
  }
</style>
