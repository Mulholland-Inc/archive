<script>
  import { Table2, Settings2, SearchX, Type, AlignLeft, CalendarDays as DateIcon, Hash, CircleCheck, ArrowUpRight, Paperclip, AtSign, DollarSign, MapPin, CircleDot, Fingerprint } from '@lucide/svelte';
  import { onMount } from 'svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import Table from '../components/Table.svelte';
  import Button from '../components/Button.svelte';
  import Text from '../components/Text.svelte';
  import Stack from '../components/Stack.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Chip from '../components/Chip.svelte';
  import Select from '../components/Select.svelte';
  import Spacer from '../components/Spacer.svelte';
  import RightSidebar from '../components/RightSidebar.svelte';
  import Heading from '../components/Heading.svelte';
  import Property from '../components/Property.svelte';
  import { app, ws, schema, viewRows } from '../lib/app.svelte.js';
  import { show } from '../lib/live.js';

  // A view, drawn the way its comment says: a table, a board grouped by its by column, or a calendar by its
  // by date. The rows are the view's own, as row-level security lets the member see them.
  let { id } = $props();

  schema.use();
  viewRows.use(() => id);

  const view = $derived(ws().schema.find((t) => t.id === id));
  const all = $derived(ws().viewRows?.[id] ?? []);
  const shown = $derived((view?.columns ?? []).filter((c) => !['id', 'access', 'created_at', 'updated_at'].includes(c.name)));
  const title = $derived(shown.find((c) => c.type === 'text') ?? shown[0]);
  const layout = $derived(view?.layout ?? 'table');
  const label = (c) => c.name.replaceAll('_', ' ').replace(/^./, (x) => x.toUpperCase());

  // Search reads every shown column; the chips are the values of the column the view groups by, or failing
  // that its first column of few values; sort is by any shown column.
  let query = $state('');
  let filter = $state(null);
  let sort = $state('');
  let open = $state(null);
  let searchEl = $state();

  const facet = $derived.by(() => {
    const few = (c) => new Set(all.map((r) => show(r[c.name])).filter(Boolean)).size;
    const by = shown.find((c) => c.name === view?.by && c.type !== 'date');
    return by ?? shown.find((c) => (c.type === 'choice' || c.type === 'text') && c !== title && few(c) > 1 && few(c) <= 8);
  });
  const values = $derived(facet ? [...new Set(all.map((r) => show(r[facet.name])).filter(Boolean))].sort() : []);
  const sorts = $derived([{ value: '', label: 'Default order' }, ...shown.map((c) => ({ value: c.name, label: label(c) }))]);
  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = all.filter((r) => (!filter || show(r[facet?.name]) === filter) && (!q || shown.some((c) => show(r[c.name]).toLowerCase().includes(q))));
    if (!sort) return out;
    return [...out].sort((a, b) => String(a[sort] ?? '').localeCompare(String(b[sort] ?? ''), undefined, { numeric: true }));
  });

  // A row opens in the side panel, its fields as properties, each with its type's icon.
  const typeIcons = { text: Type, long: AlignLeft, date: DateIcon, number: Hash, money: DollarSign, yes: CircleCheck, link: ArrowUpRight, file: Paperclip, email: AtSign, place: MapPin, choice: CircleDot, id: Fingerprint };
  function pick(r) {
    open = r;
    app.details = true;
  }

  function clear() {
    query = '';
    filter = null;
  }

  onMount(() => {
    const key = (event) => {
      if (event.key === '/' && !event.metaKey && !event.ctrlKey && !/INPUT|TEXTAREA/.test(document.activeElement?.tagName ?? '')) {
        event.preventDefault();
        searchEl?.focus();
      }
    };
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  });

  const columns = $derived(shown.map((c) => ({ key: c.name, label: label(c) })));
  const cells = $derived(rows.map((r, i) => ({ id: r.id ?? i, row: r, ...Object.fromEntries(shown.map((c) => [c.name, show(r[c.name])])) })));

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

{#snippet topbar()}
  <TopBar mobile>
    {#snippet leading()}<MobileMenu />{/snippet}
  </TopBar>
{/snippet}

{#if !view}
  {@render topbar()}
  <Page center>
    <EmptyState icon={Table2} title="View not found" />
  </Page>
{:else}
  <RightSidebar label="Row details" toggle>
  {@render topbar()}
  <Page width="wide">
    <StickyHeader title={view.name} description={view.description && view.description.toLowerCase() !== view.name.toLowerCase() ? view.description : ''}>
      {#snippet search()}
        <SearchField bind:value={query} bind:element={searchEl} placeholder={`Find in ${view.name.toLowerCase()}`} shortcut="/" />
      {/snippet}
      {#snippet actions()}
        <Button size="sm" variant="ghost" icon={Settings2} href={`/database/${view.id}`}>Details</Button>
      {/snippet}
    </StickyHeader>

    <Stack gap="6">
      <Stack direction="row" gap="2" align="center" wrap>
        {#if values.length > 1}
          <Chip selected={!filter} onclick={() => (filter = null)}>All</Chip>
          {#each values as v (v)}
            <Chip selected={filter === v} onclick={() => (filter = filter === v ? null : v)}>{v}</Chip>
          {/each}
        {/if}
        <Spacer />
        <Select bind:value={sort} options={sorts} variant="ghost" label="Sort by" placement="bottom-end" />
      </Stack>

      {#if !all.length}
        <EmptyState icon={Table2} title="Nothing here yet" />
      {:else if !rows.length}
        <EmptyState icon={SearchX} title="Nothing matches that search">
          <Button onclick={clear}>Show everything</Button>
        </EmptyState>
      {:else if layout === 'board' && view.by}
        <div class="board">
          {#each lanes as [lane, items] (lane)}
            <section class="lane">
              <header><Text size="sm" weight="medium">{lane}</Text><Text size="sm" tone="tertiary" tabular>{items.length}</Text></header>
              {#each items as r, i (r.id ?? i)}
                <button type="button" class="card" onclick={() => pick(r)}>
                  <Text size="sm" weight="medium">{show(r[title?.name])}</Text>
                  {#each shown.filter((c) => c !== title && c.name !== view.by && r[c.name] != null && r[c.name] !== '') as c (c.name)}
                    <Text size="sm" tone="secondary">{show(r[c.name])}</Text>
                  {/each}
                </button>
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
                  <button type="button" class="event" onclick={() => pick(r)}>{show(r[title?.name])}</button>
                {/each}
              </div>
            {/each}
          </div>
        </Stack>
      {:else}
        <Table {columns} rows={cells} label={view.name} onrowclick={(c) => pick(c.row)} />
      {/if}
    </Stack>
  </Page>

    {#snippet side()}
      {#if open}
        <Stack gap="4">
          <Heading level={2} size="lg" wrap="anywhere">{show(open[title?.name]) || view.name}</Heading>
          <Stack gap="0">
            {#each shown.filter((c) => c !== title) as c (c.name)}
              <Property compact label={label(c)} icon={typeIcons[c.type] ?? Type}>
                {#if open[c.name] == null || open[c.name] === ''}<Text size="sm" tone="tertiary">Empty</Text>{:else}<Text size="sm" wrap="anywhere">{show(open[c.name])}</Text>{/if}
              </Property>
            {/each}
          </Stack>
        </Stack>
      {/if}
    {/snippet}
  </RightSidebar>
{/if}

<style>
  .board {
    display: grid;
    min-width: 0;
    max-width: 100%;
    padding-block-end: var(--space-2);
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
    width: 100%;
    text-align: start;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
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

  .card:hover {
    background: var(--color-surface-raised);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .event {
    display: block;
    width: 100%;
    text-align: start;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: var(--space-0-5) var(--space-1);
    border-radius: var(--radius-sm);
    background: var(--color-fill-strong);
    font-size: var(--text-sm);
  }
</style>
