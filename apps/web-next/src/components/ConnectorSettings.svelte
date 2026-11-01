<script>
  import { SearchX, ChevronDown, Plus, ListFilter, Code } from '@lucide/svelte';
  import SettingsPane from './SettingsPane.svelte';
  import SearchField from './SearchField.svelte';
  import ConnectorCard from './ConnectorCard.svelte';
  import EmptyState from './EmptyState.svelte';
  import Button from './Button.svelte';
  import IconButton from './IconButton.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import { ws, connect } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  let { onopen, onback, backNarrow = false } = $props();

  let query = $state('');
  let categories = $state([]);
  const allCategories = $derived([...new Set(ws().connectors.map((c) => c.category))].sort());

  const matches = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return ws().connectors.filter(
      (c) =>
        (!categories.length || categories.includes(c.category)) &&
        (!q || [c.name, c.category, c.description, c.by].some((v) => v.toLowerCase().includes(q)))
    );
  });
  const narrowed = $derived(Boolean(query.trim() || categories.length));

  const flip = (list, value) => (list.includes(value) ? list.filter((v) => v !== value) : [...list, value]);
</script>

<SettingsPane {onback} {backNarrow}>
  {#snippet tools()}
    <div class="search"><SearchField bind:value={query} size="sm" placeholder="Search" label="Search connectors" /></div>
    <Menu label="Filter" placement="bottom-end">
      {#snippet trigger(props)}<IconButton icon={ListFilter} label="Filter" pressed={categories.length > 0} {...props} />{/snippet}
      <MenuLabel>Category</MenuLabel>
      {#each allCategories as c (c)}
        <MenuItem keepOpen checked={categories.includes(c)} onselect={() => (categories = flip(categories, c))}>{c}</MenuItem>
      {/each}
      {#if categories.length}
        <MenuSeparator />
        <MenuItem onselect={() => (categories = [])}>Clear filters</MenuItem>
      {/if}
    </Menu>
    <Menu label="Add" placement="bottom-end" width="sm">
      {#snippet trigger(props)}<Button variant="inverse" icon={Plus} iconEnd={ChevronDown} {...props}>Add</Button>{/snippet}
      <MenuItem icon={Code} onselect={() => toast('Custom connectors are coming soon')}>Custom connector</MenuItem>
    </Menu>
  {/snippet}
  <div class="list">
    {#if matches.length}
      <div class="grid">
        {#each matches as c (c.id)}
          <ConnectorCard connector={c} onopen={() => onopen?.(c.id)} onconnect={() => connect(c.id)} />
        {/each}
      </div>
    {:else}
      <EmptyState icon={SearchX} title={narrowed ? 'No connectors match' : 'No connectors yet'} size="sm" />
    {/if}
  </div>
</SettingsPane>

<style>
  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .search {
    flex: 1 1 var(--col-sm);
    max-width: var(--col-lg);
    min-width: 0;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-3);
  }

  @container (max-width: 36rem) {
    .grid {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  @media (max-width: 56rem) {
    .search {
      max-width: none;
    }
  }
</style>
