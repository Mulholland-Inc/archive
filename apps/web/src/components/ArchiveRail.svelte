<script>
  import { Plus } from '@lucide/svelte';
  import ArchiveRailItem from './ArchiveRailItem.svelte';
  import ArchiveIcon from './ArchiveIcon.svelte';
  import Icon from './Icon.svelte';
  import { archiveItems } from '../lib/menus.js';

  let { archives = [], current, onselect, oncreate } = $props();

  const running = (w) => w.workflows.filter((f) => f.runs?.[0] && ['running', 'waiting'].includes(f.runs[0].status)).length;
</script>

<nav class="rail" aria-label="Archives">
  <div class="list">
  {#each archives as w (w.id)}
    <ArchiveRailItem label={w.name} active={w.id === current} count={running(w)} items={() => archiveItems(w)} onclick={() => onselect?.(w.id)}>
      <ArchiveIcon icon={w.look.icon} tint={w.look.tint} seed={w.id} size="rail" />
    </ArchiveRailItem>
  {/each}
  <ArchiveRailItem label="New archive" add onclick={oncreate}>
    <Icon icon={Plus} />
  </ArchiveRailItem>
  </div>
</nav>

<style>
  .rail {
    display: flex;
    flex-direction: column;
    flex: none;
    width: var(--archive-rail);
    height: 100%;
    padding-block-end: var(--dock-space, var(--space-0));
    background: var(--color-rail);
    box-shadow: inset calc(var(--border-width) * -1) 0 0 var(--color-border-subtle);
    transition: background-color var(--duration-normal) var(--ease-out);
  }

  .list {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
    min-height: 0;
    padding-block: var(--space-3);
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-width: none;
  }
</style>
