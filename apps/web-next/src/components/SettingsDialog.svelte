<script>
  import { X } from '@lucide/svelte';
  import Dialog from './Dialog.svelte';
  import SettingsNav from './SettingsNav.svelte';
  import SearchField from './SearchField.svelte';
  import IconButton from './IconButton.svelte';

  let { open = $bindable(false), items = [], section = $bindable(null), onselect, children: pane } = $props();

  let query = $state('');
  const current = $derived(section ?? items[0]?.id);
</script>

<Dialog bind:open bare size="xl" label="Settings">
  {#snippet children({ close })}
    <div class={['layout', { listing: section === null }]}>
      <header class="bar">
        <span class="brand">Settings</span>
        <IconButton icon={X} label="Close" tooltip={false} onclick={close} />
      </header>
      <aside class="aside">
        <div class="search"><SearchField bind:value={query} size="sm" label="Search settings" /></div>
        <div class="scroll">
          <SettingsNav {items} {query} active={current} onselect={(id) => { if (!items.find((i) => i.id === id)?.href) section = id; onselect?.(id); }} />
        </div>
      </aside>
      {@render pane()}
    </div>
  {/snippet}
</Dialog>

<style>
  .layout {
    display: grid;
    grid-template-columns: var(--settings-dialog-nav) minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr);
    height: 100%;
    min-height: 0;
  }

  .bar {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-3) var(--space-3) var(--space-5);
    border-block-end: var(--border-width) solid var(--color-border-subtle);
  }

  .brand {
    font-family: var(--font-sans);
    font-size: var(--text-base);
    font-weight: var(--weight-medium);
    line-height: var(--leading-tight);
  }

  .aside {
    display: flex;
    flex-direction: column;
    min-height: 0;
    border-inline-end: var(--border-width) solid var(--color-border-subtle);
  }

  .search {
    padding: var(--space-4) var(--space-3) var(--space-2);
  }

  .scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-1) var(--space-3) var(--space-4);
  }

  @media (max-width: 56rem) {
    .layout {
      grid-template-columns: minmax(0, 1fr);
    }

    .bar {
      padding-inline-start: var(--space-4);
    }

    .aside {
      border-inline-end: none;
    }

    .layout:not(.listing) .aside,
    .listing :global(.pane) {
      display: none;
    }
  }
</style>
