<script>
  import { SearchX, ChevronDown, Plus, ListFilter, ArrowRight, Globe, Code } from '@lucide/svelte';
  import SettingsPane from './SettingsPane.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import SearchField from './SearchField.svelte';
  import ConnectorCard from './ConnectorCard.svelte';
  import EmptyState from './EmptyState.svelte';
  import Chip from './Chip.svelte';
  import Button from './Button.svelte';
  import IconButton from './IconButton.svelte';
  import Icon from './Icon.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import { ws, connect } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  let { onopen, onback, backNarrow = false } = $props();

  let tab = $state(ws().connectors.some((c) => c.connected) ? 'yours' : 'discover');
  let query = $state('');
  let categories = $state([]);
  let all = $state(false);

  const views = [
    { value: 'yours', label: 'Yours' },
    { value: 'discover', label: 'Discover' }
  ];
  const top = 6;
  const allCategories = $derived([...new Set(ws().connectors.map((c) => c.category))].sort());

  const matches = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return ws().connectors.filter(
      (c) =>
        (tab === 'discover' || c.connected) &&
        (!categories.length || categories.includes(c.category)) &&
        (!q || [c.name, c.category, c.description, c.by].some((v) => v.toLowerCase().includes(q)))
    );
  });
  const narrowed = $derived(Boolean(query.trim() || categories.length));
  const capped = $derived(tab === 'discover' && !narrowed && !all && matches.length > top);
  const shown = $derived(capped ? matches.slice(0, top) : matches);
  const heading = $derived(narrowed ? 'Results' : tab === 'yours' ? 'Connected' : 'Top connectors');

  const flip = (list, value) => (list.includes(value) ? list.filter((v) => v !== value) : [...list, value]);
</script>

<SettingsPane title="Connectors" {onback} {backNarrow}>
  {#snippet tabs()}
    <SegmentedControl bind:value={tab} options={views} label="Connectors" onchange={() => (all = false)} />
  {/snippet}
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
      <MenuItem icon={Globe} onselect={() => { tab = 'discover'; all = true; }}>Browse connectors</MenuItem>
      <MenuItem icon={Code} onselect={() => toast('Custom connectors are coming soon')}>Custom connector</MenuItem>
    </Menu>
  {/snippet}
  <div class="list">
    <div class="head">
      <div class="title">
        <h3>{heading}</h3>
        <Chip size="sm">{matches.length}</Chip>
      </div>
      {#if capped}
        <button type="button" class="more" onclick={() => (all = true)}>Show all <Icon icon={ArrowRight} size="sm" /></button>
      {/if}
    </div>
    {#if shown.length}
      <div class="grid">
        {#each shown as c (c.id)}
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

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    min-height: var(--control-md);
  }

  .title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  h3 {
    font-size: var(--text-base);
    font-weight: var(--weight-medium);
  }

  .more {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    color: var(--color-text);
    font-size: var(--text-md);
  }

  .more:hover {
    color: var(--color-text-secondary);
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
