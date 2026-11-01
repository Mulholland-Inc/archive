<script>
  import { getContext } from 'svelte';
  import VaultIcon from './VaultIcon.svelte';

  let { vault } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

<div class={['header', { collapsed }]}>
  <VaultIcon icon={vault?.look.icon} tint={vault?.look.tint} seed={vault?.id} size="lg" label={collapsed ? vault?.name : undefined} />
  <span class="lines">
    <span class="name">{vault?.name}</span>
    <span class="sub">{vault?.kind}, {vault?.members.length} {vault?.members.length === 1 ? 'member' : 'members'}</span>
  </span>
</div>

<style>
  .header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    min-width: 0;
    min-height: var(--vault-header);
    padding: var(--space-3) var(--space-3);
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

  .collapsed .lines {
    display: none;
  }
</style>
