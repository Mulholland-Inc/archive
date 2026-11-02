<script>
  import { Table2, Settings2 } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import Table from '../components/Table.svelte';
  import Button from '../components/Button.svelte';
  import Heading from '../components/Heading.svelte';
  import Text from '../components/Text.svelte';
  import Stack from '../components/Stack.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import { ws, schema, viewRows } from '../lib/app.svelte.js';
  import { show } from '../lib/live.js';

  // A view, drawn the way its comment says: a table, a board grouped by its by column, or a calendar by its
  // by date. The rows are the view's own, as row-level security lets the member see them.
  let { id } = $props();

  schema.use();
  viewRows.use(() => id);

  const view = $derived(ws().schema.find((t) => t.id === id));
  const rows = $derived(ws().viewRows?.[id] ?? []);
  const shown = $derived((view?.columns ?? []).filter((c) => !['id', 'access', 'created_at', 'updated_at'].includes(c.name)));
  const title = $derived(shown.find((c) => c.type === 'text') ?? shown[0]);
  const layout = $derived(view?.layout ?? 'table');

  const columns = $derived(shown.map((c) => ({ key: c.name, label: c.name.replaceAll('_', ' ').replace(/^./, (s) => s.toUpperCase()) })));
  const cells = $derived(rows.map((r, i) => ({ id: r.id ?? i, ...Object.fromEntries(shown.map((c) => [c.name, show(r[c.name])])) })));

  const lanes = $derived.by(() => {
    const by = view?.by;
    const out = new Map();
    for (const r of rows) {
      const k = r[by] == null || r[by] === '' ? 'None' : show(r[by]);
      if (!out.has(k)) out.set(k, []);
      out.get(k).push(r);
    }
    return [...out];
  });

  // The calendar is the month of the earliest upcoming row, or this month.
  const month = $derived.by(() => {
    const days = rows.map((r) => r[view?.by]).filter(Boolean).map((d) => new Date(d));
    const now = new Date();
    const next = days.filter((d) => d >= new Date(now.getFullYear(), now.getMonth(), 1)).sort((a, b) => a - b)[0] ?? now;
    return new Date(next.getFullYear(), next.getMonth(), 1);
  });
  const weeks = $derived.by(() => {
    const first = new Date(month);
    first.setDate(1 - ((first.getDay() + 6) % 7));
    return Array.from({ length: 6 }, (_, w) => Array.from({ length: 7 }, (_, d) => new Date(first.getFullYear(), first.getMonth(), first.getDate() + w * 7 + d)));
  });
  const key = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  const onDay = $derived.by(() => {
    const out = new Map();
    for (const r of rows) {
      const v = r[view?.by];
      if (!v) continue;
      const k = String(v).slice(0, 10);
      if (!out.has(k)) out.set(k, []);
      out.get(k).push(r);
    }
    return out;
  });
</script>

<TopBar>
  {#snippet leading()}
    <MobileMenu />
    <Breadcrumbs inset items={[{ label: view?.name ?? 'Not found' }]} />
  {/snippet}
  {#snippet trailing()}
    {#if view}<Button size="sm" variant="ghost" icon={Settings2} href={`/database/${view.id}`}>Details</Button>{/if}
  {/snippet}
</TopBar>

{#if !view}
  <Page center>
    <EmptyState icon={Table2} title="View not found" />
  </Page>
{:else}
  <Page width="wide">
    <Stack gap="6">
      <Stack gap="1">
        <Heading level={1} size="2xl" wrap="anywhere">{view.name}</Heading>
        {#if view.description}<Text tone="secondary">{view.description}</Text>{/if}
      </Stack>

      {#if !rows.length}
        <EmptyState icon={Table2} title="Nothing here yet" />
      {:else if layout === 'board' && view.by}
        <div class="board">
          {#each lanes as [lane, items] (lane)}
            <section class="lane">
              <header><Text size="sm" weight="medium">{lane}</Text><Text size="sm" tone="tertiary" tabular>{items.length}</Text></header>
              {#each items as r, i (r.id ?? i)}
                <article class="card">
                  <Text size="sm" weight="medium">{show(r[title?.name])}</Text>
                  {#each shown.filter((c) => c !== title && c.name !== view.by && r[c.name] != null && r[c.name] !== '') as c (c.name)}
                    <Text size="sm" tone="secondary">{show(r[c.name])}</Text>
                  {/each}
                </article>
              {/each}
            </section>
          {/each}
        </div>
      {:else if layout === 'calendar' && view.by}
        <Stack gap="2">
          <Text weight="medium">{month.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}</Text>
          <div class="calendar">
            {#each ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'] as d (d)}<Text size="sm" tone="tertiary">{d}</Text>{/each}
            {#each weeks.flat() as d (key(d))}
              <div class={['day', { other: d.getMonth() !== month.getMonth() }]}>
                <Text size="sm" tone="tertiary" tabular>{d.getDate()}</Text>
                {#each onDay.get(key(d)) ?? [] as r, i (r.id ?? i)}
                  <span class="event">{show(r[title?.name])}</span>
                {/each}
              </div>
            {/each}
          </div>
        </Stack>
      {:else}
        <Table {columns} rows={cells} label={view.name} />
      {/if}
    </Stack>
  </Page>
{/if}

<style>
  .board {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: minmax(14rem, 1fr);
    gap: var(--space-3);
    overflow-x: auto;
    align-items: start;
  }

  .lane {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-2);
    border-radius: var(--radius-lg);
    background: var(--color-fill);
  }

  .lane header {
    display: flex;
    justify-content: space-between;
    padding: var(--space-1) var(--space-1-5);
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .calendar {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    gap: var(--space-px);
  }

  .day {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-height: 6rem;
    padding: var(--space-1-5);
    border-radius: var(--radius-sm);
    background: var(--color-fill);
    min-width: 0;
  }

  .other {
    opacity: 0.5;
  }

  .event {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: var(--space-0-5) var(--space-1);
    border-radius: var(--radius-sm);
    background: var(--color-fill-strong);
    font-size: var(--text-sm);
  }
</style>
