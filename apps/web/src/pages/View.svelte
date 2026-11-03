<script>
  import { Table2, Settings2, SearchX } from '@lucide/svelte';
  import { go } from '../lib/router.svelte.js';
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
  import RowLink from '../components/RowLink.svelte';
  import { ws, schema, viewRows, linkOf, tableOf, shownOf, titleOf, cellText, renameView, isArchiveOwner } from '../lib/app.svelte.js';

  // A view, drawn the way its comment says: a table, a board grouped by its by column, or a calendar by its
  // by date. The rows are the view's own, as row-level security lets the member see them. Each row is called
  // by its name, and each link by the name of the row it names, never by an id.
  let { id } = $props();

  schema.use();
  viewRows.use(() => id);

  const view = $derived(tableOf(id));
  const all = $derived(ws().viewRows?.[id] ?? []);
  const shown = $derived(shownOf(view));
  const layout = $derived(view?.layout ?? 'table');
  // Archive's own tables are shown as views are, but they are Archive's: not renamed, and with no page of DDL.
  const mine = $derived(Boolean(view) && ws().schema.includes(view));
  const text = (c, r) => cellText(view, c, r);
  const label = (c) => c.name.replaceAll('_', ' ').replace(/^./, (x) => x.toUpperCase());

  // Search reads every shown column; the chips are the values of the column the view groups by, or failing
  // that its first column of few values that repeat: one whose rows each have their own, like an id from
  // another service, filters nothing. Sort is by any shown column.
  let query = $state('');
  let filter = $state(null);
  let sort = $state('');
  let searchEl = $state();

  const facet = $derived.by(() => {
    const vals = (c) => all.map((r) => text(c, r)).filter(Boolean);
    const few = (c) => {
      const v = vals(c);
      const n = new Set(v).size;
      return n > 1 && n <= 8 && n < v.length;
    };
    const by = shown.find((c) => c.name === view?.by && c.type !== 'date');
    return by ?? shown.find((c) => (c.type === 'choice' || c.type === 'text') && few(c));
  });
  const values = $derived(facet ? [...new Set(all.map((r) => text(facet, r)).filter(Boolean))].sort() : []);
  const sorts = $derived([{ value: '', label: 'Default order' }, { value: 'name', label: 'Name' }, ...shown.map((c) => ({ value: c.name, label: label(c) }))]);
  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = all.filter((r) => (!filter || text(facet, r) === filter) && (!q || titleOf(r).toLowerCase().includes(q) || shown.some((c) => text(c, r).toLowerCase().includes(q))));
    if (!sort) return out;
    const key = (r) => (sort === 'name' ? titleOf(r) : text(shown.find((c) => c.name === sort), r));
    return [...out].sort((a, b) => key(a).localeCompare(key(b), undefined, { numeric: true }));
  });

  // A row opens its own page, like a table's.
  const pick = (r) => r.id && go(`/view/${id}/${r.id}`);

  // The board scrolls sideways; a vertical wheel over it scrolls the page, which the browser would otherwise
  // keep from the page once the board, a scroller too, catches it.
  function wheel(node) {
    const on = (e) => {
      if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return;
      const page = node.closest('.scroll');
      if (!page) return;
      e.preventDefault();
      page.scrollBy({ top: e.deltaY * (e.deltaMode === 1 ? 16 : 1) });
    };
    node.addEventListener('wheel', on, { passive: false });
    return () => node.removeEventListener('wheel', on);
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

  const columns = $derived([{ key: 'name', label: 'Name' }, ...shown.map((c) => ({ key: c.name, label: label(c) }))]);
  const cells = $derived(rows.map((r, i) => ({ id: r.id ?? i, row: r, name: titleOf(r), ...Object.fromEntries(shown.map((c) => [c.name, text(c, r)])) })));
  const byCol = $derived(shown.find((c) => c.name === view?.by));

  const lanes = $derived.by(() => {
    const by = view?.by;
    const out = new Map();
    for (const r of rows) {
      const k = r[by] == null || r[by] === '' ? 'None' : byCol ? text(byCol, r) : String(r[by]);
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
  {@render topbar()}
  <Page width="wide">
    <StickyHeader title={view.name} renameLabel="Rename view" onrename={mine && isArchiveOwner() ? (name) => renameView(view.id, name) : undefined} description={view.description && view.description.toLowerCase() !== view.name.toLowerCase() ? view.description : ''}>
      {#snippet search()}
        <SearchField bind:value={query} bind:element={searchEl} placeholder={`Find in ${view.name.toLowerCase()}`} shortcut="/" />
      {/snippet}
      {#snippet actions()}
        {#if mine}<Button size="sm" variant="ghost" icon={Settings2} href={`/database/${view.id}`}>Details</Button>{/if}
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
        <div class="board" {@attach wheel}>
          {#each lanes as [lane, items] (lane)}
            <section class="lane">
              <header><Text size="sm" weight="medium">{lane}</Text><Text size="sm" tone="tertiary" tabular>{items.length}</Text></header>
              {#each items as r, i (r.id ?? i)}
                <!-- The card opens its row; its chips, above that, open the rows they name. -->
                <div class="card">
                  <button type="button" class="hit" aria-label={titleOf(r)} onclick={() => pick(r)}></button>
                  <Text size="sm" weight="medium">{titleOf(r)}</Text>
                  {#each shown.filter((c) => c.name !== view.by && r[c.name] != null && r[c.name] !== '') as c (c.name)}
                    {#if linkOf(view, c)}<span class="chips raised"><RowLink table={linkOf(view, c)} value={r[c.name]} /></span>{:else}<Text size="sm" tone="secondary">{text(c, r)}</Text>{/if}
                  {/each}
                </div>
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
                  <button type="button" class="event" onclick={() => pick(r)}>{titleOf(r)}</button>
                {/each}
              </div>
            {/each}
          </div>
        </Stack>
      {:else}
        <Table {columns} rows={cells} label={view.name} onrowclick={(c) => pick(c.row)}>
          {#snippet cell(row, col)}
            {@const c = shown.find((x) => x.name === col.key)}
            {#if c && linkOf(view, c)}<span class="chips"><RowLink table={linkOf(view, c)} value={row.row[c.name]} /></span>{:else}{row[col.key]}{/if}
          {/snippet}
        </Table>
      {/if}
    </Stack>
  </Page>

{/if}

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

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
    position: relative;
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

  .hit {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    cursor: pointer;
  }

  .raised {
    position: relative;
    z-index: var(--z-raised);
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
    /* A long name wraps onto more lines rather than end in an ellipsis: the day is narrow, and the name is the
       only thing an entry shows. */
    overflow-wrap: anywhere;
    line-height: var(--leading-tight);
    padding: var(--space-0-5) var(--space-1);
    border-radius: var(--radius-sm);
    background: var(--color-fill-strong);
    font-size: var(--text-sm);
  }
</style>
