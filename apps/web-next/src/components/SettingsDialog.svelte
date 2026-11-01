<script>
  import { X } from '@lucide/svelte';
  import Dialog from './Dialog.svelte';
  import SettingsNav from './SettingsNav.svelte';
  import SettingsPane from './SettingsPane.svelte';
  import IconButton from './IconButton.svelte';
  import { uid } from '../lib/ids.js';

  let { open = $bindable(false), items = [], section = $bindable(null), title, onback, onselect, children } = $props();

  const titleId = uid('settings');
  const current = $derived(section ?? items[0]?.id);
</script>

<Dialog bind:open bare size="xl" label="Settings">
  {#snippet children({ close })}
    <div class={['layout', { listing: section === null }]}>
      <aside class="aside">
        <div class="head">
          <span class="brand">Settings</span>
          <span class="close"><IconButton icon={X} label="Close" size="sm" tooltip={false} onclick={close} /></span>
        </div>
        <SettingsNav {items} active={current} onselect={(id) => { section = id; onselect?.(id); }} />
      </aside>
      <SettingsPane {title} {titleId} onback={onback ?? (() => (section = null))} backNarrow={!onback} onclose={close}>
        {@render children()}
      </SettingsPane>
    </div>
  {/snippet}
</Dialog>

<style>
  .layout {
    display: grid;
    grid-template-columns: var(--settings-dialog-nav) minmax(0, 1fr);
    height: 100%;
    min-height: 0;
  }

  .aside {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-3) var(--space-2);
    background: var(--color-surface-sunken);
    border-inline-end: var(--border-width) solid var(--color-border-subtle);
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: var(--control-md);
    padding-inline: var(--space-2);
  }

  .brand {
    font-family: var(--font-sans);
    font-size: var(--text-base);
    font-weight: var(--weight-semibold);
    line-height: var(--leading-tight);
  }

  .close {
    display: none;
  }

  @media (max-width: 56rem) {
    .layout {
      grid-template-columns: minmax(0, 1fr);
    }

    .aside {
      padding: var(--space-3) var(--space-2);
      background: var(--color-surface-raised);
      border-inline-end: none;
    }

    .head {
      padding-inline: var(--space-2) var(--space-0);
    }

    .close {
      display: flex;
    }

    .layout:not(.listing) .aside,
    .listing :global(.pane) {
      display: none;
    }
  }
</style>
