<script>
  import { SearchX, ListFilter } from '@lucide/svelte';
  import SettingsPane from './SettingsPane.svelte';
  import PaneHeader from './PaneHeader.svelte';
  import SearchField from './SearchField.svelte';
  import ConnectorCard from './ConnectorCard.svelte';
  import OwnerNotice from './OwnerNotice.svelte';
  import EmptyState from './EmptyState.svelte';
  import IconButton from './IconButton.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import { archiveIntegrations, isArchiveOwner } from '../lib/app.svelte.js';

  let { onopen, onback } = $props();

  let query = $state('');
  let categories = $state([]);
  let status = $state('all');
  const all = $derived(archiveIntegrations());
  const owner = $derived(isArchiveOwner());
  const allCategories = $derived([...new Set(all.map((c) => c.category))]);

  const matches = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return all.filter(
      (c) =>
        (!categories.length || categories.includes(c.category)) &&
        (status === 'all' || (status === 'connected') === c.connected) &&
        (!q || [c.name, c.category, c.description, c.by, c.account].some((v) => v.toLowerCase().includes(q)))
    );
  });
  const filtered = $derived(categories.length > 0 || status !== 'all');

  const flip = (list, value) => (list.includes(value) ? list.filter((v) => v !== value) : [...list, value]);
</script>

<SettingsPane {onback} backNarrow>
  <PaneHeader title="Integrations" description="Set up once for the whole archive, so everyone’s work and sign-in run through them. Only owners can add or change them." art="connectors">
    {#snippet search()}
      <div class="search"><SearchField bind:value={query} size="sm" placeholder="Search" label="Search integrations" /></div>
      <Menu label="Filter" placement="bottom-end">
        {#snippet trigger(props)}<IconButton icon={ListFilter} label="Filter" pressed={filtered} {...props} />{/snippet}
        <MenuLabel>Status</MenuLabel>
        <MenuItem keepOpen checked={status === 'connected'} onselect={() => (status = status === 'connected' ? 'all' : 'connected')}>Connected</MenuItem>
        <MenuItem keepOpen checked={status === 'available'} onselect={() => (status = status === 'available' ? 'all' : 'available')}>Not connected</MenuItem>
        <MenuSeparator />
        <MenuLabel>Category</MenuLabel>
        {#each allCategories as c (c)}
          <MenuItem keepOpen checked={categories.includes(c)} onselect={() => (categories = flip(categories, c))}>{c}</MenuItem>
        {/each}
        {#if filtered}
          <MenuSeparator />
          <MenuItem onselect={() => { categories = []; status = 'all'; }}>Clear filters</MenuItem>
        {/if}
      </Menu>
    {/snippet}
  </PaneHeader>
  {#if !owner}<OwnerNotice what="integrations" />{/if}
  {#if matches.length}
    <div class="grid">
      {#each matches as c (c.id)}
        <ConnectorCard
          connector={c}
          onopen={() => onopen?.(c.id)}
          onconnect={owner ? () => onopen?.(c.id) : undefined}
        />
      {/each}
    </div>
  {:else}
    <EmptyState icon={SearchX} title="No integrations match" size="sm" />
  {/if}
</SettingsPane>

<style>
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

  @container (max-width: 32rem) {
    .search {
      max-width: none;
    }
  }
</style>
