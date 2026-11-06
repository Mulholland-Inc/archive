<script>
  import { Table2, Settings2, SearchX, ListFilter, ChevronLeft, ChevronRight } from '@lucide/svelte';
  import IconButton from '../components/IconButton.svelte';
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
  import Filter from '../components/Filter.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import Spacer from '../components/Spacer.svelte';
  import RowLink from '../components/RowLink.svelte';
  import { ws, schema, viewRows, viewState, setView, filterKind, VIEW_PAGE, linkOf, tableOf, shownOf, titleOf, cellText, renameView, atLeast } from '../lib/app.svelte.js';
  import { layoutOf } from '../layouts/index.js';

  let { id } = $props();

  schema.use();
  viewRows.use(() => id);

  const view = $derived(tableOf(id));
  const shown = $derived(shownOf(view));
  const layout = $derived(layoutOf(view));
  const mine = $derived(Boolean(view) && ws().schema.includes(view));
  const text = (c, r) => cellText(view, c, r);
  const label = (c) => c.name.replaceAll('_', ' ').replace(/^./, (x) => x.toUpperCase());

  let searchEl = $state();
  let added = $state([]);

  const state = $derived(viewState(id));
  const totals = $derived(ws().viewTotals?.[id]);
  const rows = $derived(ws().viewRows?.[id] ?? []);
  const active = $derived(Boolean(state.query.trim()) || Object.keys(state.filters).length > 0);
  const filterable = $derived([...shown, ...(view?.columns ?? []).filter((c) => c.name === 'name')].filter(filterKind));
  const first = $derived([...filterable.filter((c) => c.name === view?.by), ...filterable.filter((c) => c.name !== view?.by)].slice(0, 6));
  const bar = $derived(filterable.filter((c) => first.includes(c) || added.includes(c.name) || state.filters[c.name]));
  const more = $derived(filterable.filter((c) => !bar.includes(c)));
  const sorts = $derived([{ value: '', label: 'Default order' }, { value: 'name', label: 'Name' }, ...shown.map((c) => ({ value: c.name, label: label(c) }))]);

  const pick = (r) => r.id && go(`/view/${id}/${r.id}`);

  function clear() {
    added = [];
    setView(id, { query: '', filters: {} });
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
    <StickyHeader title={view.name} renameLabel="Rename view" onrename={mine && atLeast('manager') ? (name) => renameView(view.id, name) : undefined} description={view.description && view.description.toLowerCase() !== view.name.toLowerCase() ? view.description : ''}>
      {#snippet search()}
        <SearchField bind:value={() => state.query, (query) => setView(id, { query }, 250)} bind:element={searchEl} placeholder={`Find in ${view.name.toLowerCase()}`} shortcut="/" />
      {/snippet}
      {#snippet actions()}
        {#if mine}<Button size="sm" variant="ghost" icon={Settings2} href={`/database/${view.id}`}>Details</Button>{/if}
      {/snippet}
    </StickyHeader>

    <Stack gap="6">
      <Stack direction="row" gap="2" align="center" wrap>
        {#each bar as c (c.name)}
          <Filter {id} column={c} label={label(c)} {text} />
        {/each}
        {#if more.length}
          <Menu label="More filters">
            {#snippet trigger(props)}
              <Button size="sm" variant="ghost" icon={ListFilter} {...props}>More</Button>
            {/snippet}
            {#each more as c (c.name)}
              <MenuItem onselect={() => (added = [...added, c.name])}>{label(c)}</MenuItem>
            {/each}
          </Menu>
        {/if}
        {#if active}<Button size="sm" variant="ghost" onclick={clear}>Clear</Button>{/if}
        <Spacer />
        <Menu bind:value={() => state.sort, (sort) => setView(id, { sort })} options={sorts} variant="ghost" label="Sort by" />
      </Stack>

      {#if totals?.error}
        <EmptyState icon={SearchX} title={totals.error} />
      {:else if !rows.length && !active}
        {#if totals}<EmptyState icon={Table2} title="Nothing here yet" />{/if}
      {:else if !rows.length}
        <EmptyState icon={SearchX} title="Nothing matches that search">
          <Button onclick={clear}>Show everything</Button>
        </EmptyState>
      {:else if layout.component && (!layout.by || view.by)}
        {#await layout.component() then m}<m.default {view} {rows} {shown} {text} {pick} />{/await}
      {:else}
        <Table {columns} rows={cells} label={view.name} onrowclick={(c) => pick(c.row)}>
          {#snippet cell(row, col)}
            {@const c = shown.find((x) => x.name === col.key)}
            {#if c && linkOf(view, c)}<span class="chips"><RowLink table={linkOf(view, c)} value={row.row[c.name]} /></span>{:else}{row[col.key]}{/if}
          {/snippet}
        </Table>
      {/if}
      {#if totals?.total}
        <Stack direction="row" gap="2" align="center">
          <Spacer />
          <span class="total">{((state.page - 1) * VIEW_PAGE + 1).toLocaleString()}–{Math.min(state.page * VIEW_PAGE, totals.total).toLocaleString()} of {totals.total.toLocaleString()}</span>
          <IconButton icon={ChevronLeft} label="Previous page" disabled={state.page === 1} onclick={() => setView(id, { page: state.page - 1 })} />
          <IconButton icon={ChevronRight} label="Next page" disabled={state.page * VIEW_PAGE >= totals.total} onclick={() => setView(id, { page: state.page + 1 })} />
        </Stack>
      {/if}
    </Stack>
  </Page>

{/if}

<style>
  .total {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }
</style>
