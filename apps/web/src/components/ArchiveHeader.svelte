<script>
  import { getContext } from 'svelte';
  import { Settings } from '@lucide/svelte';
  import ArchiveIcon from './ArchiveIcon.svelte';
  import Icon from './Icon.svelte';

  let { archive, onclick } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

<button type="button" class={['header', { collapsed }]} {onclick} aria-label={collapsed ? `${archive?.name} settings` : undefined}>
  <ArchiveIcon icon={archive?.look.icon} tint={archive?.look.tint} seed={archive?.id} size="lg" />
  <span class="lines">
    <span class="name">{archive?.name}</span>
    <span class="sub">{archive?.kind}, {archive?.members.length} {archive?.members.length === 1 ? 'member' : 'members'}</span>
  </span>
  <span class="gear"><Icon icon={Settings} size="sm" /></span>
</button>

<style>
  .header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    min-width: 0;
    min-height: var(--archive-header);
    padding: var(--space-3) var(--space-3);
    border-radius: var(--radius-lg);
    text-align: start;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .header:hover {
    background: var(--color-fill);
  }

  .gear {
    display: flex;
    flex: none;
    color: var(--color-text-tertiary);
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .header:hover .gear,
  .header:focus-visible .gear {
    opacity: 1;
  }

  .collapsed {
    justify-content: center;
    padding: var(--space-3) var(--space-0);
    gap: var(--space-0);
  }

  .lines {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    line-height: var(--leading-snug);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }

  .collapsed :is(.lines, .gear) {
    display: none;
  }
</style>
