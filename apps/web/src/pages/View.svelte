<script>
  import { Table2, Settings2, SearchX } from '@lucide/svelte';
  import { go } from '../lib/router.svelte.js';
  import { onMount } from 'svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import Table from '../components/Table.svelte';
  import Button from '../components/Button.svelte';
  import Stack from '../components/Stack.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Chip from '../components/Chip.svelte';
  import Menu from '../components/Menu.svelte';
  import Spacer from '../components/Spacer.svelte';
  import RowLink from '../components/RowLink.svelte';
  import { ws, schema, viewRows, linkOf, tableOf, shownOf, titleOf, cellText, renameView, isArchiveOwner } from '../lib/app.svelte.js';
  import { layoutOf } from '../layouts/index.js';

  // A view, drawn the way its comment's layout says (../layouts), under the same header whatever it is. The
  // rows are the view's own, as row-level security lets the member see them. Each row is called by its name,
  // and each link by the name of the row it names, never by an id.
  let { id } = $props();

  schema.use();
  viewRows.use(() => id);

  const view = $derived(tableOf(id));
  const all = $derived(ws().viewRows?.[id] ?? []);
  const shown = $derived(shownOf(view));
  const layout = $derived(layoutOf(view));
  // Archive's own tables are shown as views are, but they are Archive's: not renamed, and with no page of DDL.
  const mine = $derived(Boolean(view) && ws().schema.includes(view));
  const text = (c, r) => cellText(view, c, r);
  const label = (c) => c.name.replaceAll('_', ' ').replace(/^./, (x) => x.toUpperCase());

  // Search reads every shown column; the chips are the values of the column the view groups by, unless it is a
  // date, place or number, or failing that its first column of few values that repeat: one whose rows each have
  // their own, like an id from another service, filters nothing. Sort is by any shown column.
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
    const by = shown.find((c) => c.name === view?.by && !['date', 'place', 'number', 'money'].includes(c.type));
    return by ?? shown.find((c) => (c.type === 'choice' || c.type === 'text') && few(c));
  });
  const values = $derived(facet ? [...new Set(all.map((r) => text(facet, r)).filter(Boolean))].sort() : []);
  const sorts = $derived([{ value: '', label: 'Default order' }, { value: 'name', label: 'Name' }, ...shown.map((c) => ({ value: c.name, label: label(c) }))]);
  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = all.filter((r) => (!filter || text(facet, r) === filter) && (!q || titleOf(r, view).toLowerCase().includes(q) || shown.some((c) => text(c, r).toLowerCase().includes(q))));
    if (!sort) return out;
    const key = (r) => (sort === 'name' ? titleOf(r, view) : text(shown.find((c) => c.name === sort), r));
    return [...out].sort((a, b) => key(a).localeCompare(key(b), undefined, { numeric: true }));
  });

  // A row opens its own page, like a table's.
  const pick = (r) => r.id && go(`/view/${id}/${r.id}`);

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
  const cells = $derived(rows.map((r, i) => ({ id: r.id ?? i, row: r, name: titleOf(r, view), ...Object.fromEntries(shown.map((c) => [c.name, text(c, r)])) })));
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
        <Menu bind:value={sort} options={sorts} variant="ghost" label="Sort by" />
      </Stack>

      {#if !all.length}
        <EmptyState icon={Table2} title="Nothing here yet" />
      {:else if !rows.length}
        <EmptyState icon={SearchX} title="Nothing matches that search">
          <Button onclick={clear}>Show everything</Button>
        </EmptyState>
      {:else if layout.component && (!layout.by || view.by)}
        <layout.component {view} {rows} {shown} {text} {pick} />
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
</style>
