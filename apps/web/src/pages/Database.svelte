<script>
  import { onMount } from 'svelte';
  import { Box, Database as DatabaseIcon, SearchX } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Chip from '../components/Chip.svelte';
  import Select from '../components/Select.svelte';
  import Spacer from '../components/Spacer.svelte';
  import Table from '../components/Table.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, schema, counts, workflows } from '../lib/app.svelte.js';

  schema.use();
  counts.use();
  workflows.use();
  import { go } from '../lib/router.svelte.js';
  import { tableItems } from '../lib/menus.js';

  const filters = [
    { value: 'all', label: 'All' },
    { value: 'linked', label: 'Linked' },
    { value: 'automated', label: 'Used by workflows' },
    { value: 'shared', label: 'Shared' }
  ];
  const sorts = [
    { value: 'name', label: 'Name' },
    { value: 'rows', label: 'Most rows' },
    { value: 'columns', label: 'Most columns' }
  ];
  const columns = [
    { key: 'name', label: 'Table' },
    { key: 'links', label: 'Links to', muted: true, width: 'lg', mobile: 'secondary' },
    { key: 'rows', label: 'Rows', width: 'sm', align: 'end', mobile: 'trailing' },
    { key: 'columns', label: 'Columns', muted: true, width: 'sm', align: 'end', mobile: 'hidden' },
    { key: 'updated', label: 'Updated', muted: true, width: 'sm', mobile: 'hidden' }
  ];

  let query = $state('');
  let filter = $state('all');
  let sort = $state('name');
  let searchEl = $state();

  const types = $derived(ws().schema);
  const nameOf = (id) => types.find((t) => t.id === id)?.name ?? id;
  const linksOf = (t) => [...new Set(t.links.map((l) => l.to))].map(nameOf);
  const shared = (t) => ws().shares.some((s) => s.kind === 'table' && s.target === t.id);
  const keep = {
    all: () => true,
    linked: (t) => t.links.length || types.some((o) => o.links.some((l) => l.to === t.id)),
    automated: (t) => t.workflows.length,
    shared
  };
  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = types.filter((t) => keep[filter](t) && (!q || t.name.toLowerCase().includes(q) || t.columns.some((c) => c.name.toLowerCase().includes(q))));
    if (sort === 'rows') return [...out].sort((a, b) => (b.rows ?? 0) - (a.rows ?? 0));
    if (sort === 'columns') return [...out].sort((a, b) => b.columns.length - a.columns.length);
    return [...out].sort((a, b) => a.name.localeCompare(b.name));
  });

  function clear() {
    query = '';
    filter = 'all';
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
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <StickyHeader title="Database" art="bg-database">
    {#snippet search()}
      <SearchField bind:value={query} bind:element={searchEl} placeholder="Find a table or column" shortcut="/" />
    {/snippet}
  </StickyHeader>

  <Stack direction="row" gap="2" align="center" wrap>
    {#each filters as f (f.value)}
      <Chip selected={filter === f.value} onclick={() => (filter = f.value)}>{f.label}</Chip>
    {/each}
    <Spacer />
    <Select bind:value={sort} options={sorts} variant="ghost" label="Sort by" placement="bottom-end" />
  </Stack>

  {#if !types.length}
    <EmptyState icon={DatabaseIcon} title="No tables yet" />
  {:else if !rows.length}
    <EmptyState icon={SearchX} title="Nothing matches that search">
      <Button onclick={clear}>Show all tables</Button>
    </EmptyState>
  {:else}
    <Table {columns} {rows} label="Tables" cards onrowclick={(t) => go(`/database/${t.id}`)} menu={tableItems}>
      {#snippet cell(row, col)}
        {#if col.key === 'name'}
          <Stack as="span" direction="row" gap="3" align="center">
            <Icon icon={Box} size="md" tone="tertiary" />
            <Text weight="medium" truncate>{row.name}</Text>
          </Stack>
        {:else if col.key === 'rows'}
          <Text tabular>{row.rows?.toLocaleString('en-US') ?? '—'}</Text>
        {:else if col.key === 'columns'}
          <Text tone="secondary" tabular>{row.columns.length}</Text>
        {:else if col.key === 'links'}
          <Text tone="secondary" truncate>{linksOf(row).join(', ') || 'None'}</Text>
        {:else if col.key === 'updated'}
          {row.updated}
        {/if}
      {/snippet}
    </Table>
  {/if}
</Page>
