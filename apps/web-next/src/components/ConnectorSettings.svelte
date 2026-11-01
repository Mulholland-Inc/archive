<script>
  import { SearchX, ChevronDown } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Grid from './Grid.svelte';
  import SearchField from './SearchField.svelte';
  import ConnectorCard from './ConnectorCard.svelte';
  import EmptyState from './EmptyState.svelte';
  import Chip from './Chip.svelte';
  import Button from './Button.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import { ws, connect } from '../lib/app.svelte.js';

  let { onopen } = $props();

  let query = $state('');
  let statuses = $state([]);
  let categories = $state([]);
  let sort = $state('popular');

  const statusOptions = [
    { value: 'connected', label: 'Connected' },
    { value: 'available', label: 'Not connected' }
  ];
  const sorts = [
    { value: 'popular', label: 'Popular' },
    { value: 'name', label: 'Name' },
    { value: 'connected', label: 'Connected first' }
  ];
  const allCategories = $derived([...new Set(ws().connectors.map((c) => c.category))].sort());
  const active = $derived([...statuses.map((v) => ({ kind: 'status', value: v, label: statusOptions.find((o) => o.value === v).label })), ...categories.map((v) => ({ kind: 'category', value: v, label: v }))]);

  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const list = ws().connectors.filter(
      (c) =>
        (!statuses.length || statuses.includes(c.connected ? 'connected' : 'available')) &&
        (!categories.length || categories.includes(c.category)) &&
        (!q || c.name.toLowerCase().includes(q) || c.category.toLowerCase().includes(q) || c.description.toLowerCase().includes(q))
    );
    if (sort === 'name') return list.toSorted((a, b) => a.name.localeCompare(b.name));
    if (sort === 'connected') return list.toSorted((a, b) => Number(b.connected) - Number(a.connected));
    return list;
  });

  const flip = (list, value) => (list.includes(value) ? list.filter((v) => v !== value) : [...list, value]);

  function remove(filter) {
    if (filter.kind === 'status') statuses = statuses.filter((v) => v !== filter.value);
    else categories = categories.filter((v) => v !== filter.value);
  }
</script>

<Stack gap="4">
  <SearchField bind:value={query} placeholder="Search connectors" />
  <Stack direction="row" gap="2" align="center" justify="between" wrap>
    <Stack direction="row" gap="2" align="center" wrap>
      {#if active.length}
        {#each active as filter (filter.kind + filter.value)}
          <Chip size="sm" onremove={() => remove(filter)}>{filter.label}</Chip>
        {/each}
      {:else}
        <Chip size="sm">All connectors</Chip>
      {/if}
    </Stack>
    <Stack direction="row" gap="2" align="center">
      <Menu label="Filter by" placement="bottom-end">
        {#snippet trigger(props)}<Button size="sm" iconEnd={ChevronDown} {...props}>Filter by</Button>{/snippet}
        <MenuLabel>Status</MenuLabel>
        {#each statusOptions as o (o.value)}
          <MenuItem keepOpen checked={statuses.includes(o.value)} onselect={() => (statuses = flip(statuses, o.value))}>{o.label}</MenuItem>
        {/each}
        <MenuSeparator />
        <MenuLabel>Category</MenuLabel>
        {#each allCategories as c (c)}
          <MenuItem keepOpen checked={categories.includes(c)} onselect={() => (categories = flip(categories, c))}>{c}</MenuItem>
        {/each}
      </Menu>
      <Menu label="Sort by" placement="bottom-end" width="sm">
        {#snippet trigger(props)}<Button size="sm" iconEnd={ChevronDown} {...props}>Sort by</Button>{/snippet}
        {#each sorts as o (o.value)}
          <MenuItem checked={sort === o.value} onselect={() => (sort = o.value)}>{o.label}</MenuItem>
        {/each}
      </Menu>
    </Stack>
  </Stack>
  {#if shown.length}
    <Grid columns="2" gap="3">
      {#each shown as c (c.id)}
        <ConnectorCard connector={c} onopen={() => onopen?.(c.id)} onconnect={() => connect(c.id)} />
      {/each}
    </Grid>
  {:else}
    <EmptyState icon={SearchX} title="No connectors match" size="sm" />
  {/if}
</Stack>
