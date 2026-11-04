<script>
  import { getContext } from 'svelte';
  import { ChevronDown } from '@lucide/svelte';
  import ArchiveIcon from './ArchiveIcon.svelte';
  import Icon from './Icon.svelte';

  let { archive, ...rest } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

<button type="button" class={['header', { collapsed }]} aria-label={collapsed ? archive?.name : undefined} {...rest}>
  <ArchiveIcon icon={archive?.look.icon} tint={archive?.look.tint} seed={archive?.id} size="lg" />
  <span class="lines">
    <span class="name">{archive?.name}</span>
    <span class="sub">{archive?.kind}, {archive?.members.length} {archive?.members.length === 1 ? 'member' : 'members'}</span>
  </span>
  <span class="chevron"><Icon icon={ChevronDown} size="md" /></span>
</button>

<style>
  .header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    min-width: 0;
    min-height: var(--archive-header);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
    text-align: start;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .header:hover,
  .header[aria-expanded='true'] {
    background: var(--color-fill);
  }

  .collapsed {
    justify-content: center;
    padding: var(--space-2) var(--space-0);
    gap: var(--space-0);
    background: transparent;
    box-shadow: none;
  }

  .chevron {
    display: flex;
    flex: none;
    color: var(--color-text-secondary);
  }

  .collapsed .chevron {
    display: none;
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

  .collapsed .lines {
    display: none;
  }
</style>
