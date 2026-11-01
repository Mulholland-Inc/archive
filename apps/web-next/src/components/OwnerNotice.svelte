<script>
  import { Lock } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import { ws } from '../lib/app.svelte.js';

  let { what = 'this', ask = 'to connect or change one' } = $props();
  const owners = $derived(ws().members.filter((m) => m.role === 'owner' && m.status === 'active').map((m) => m.name));
  const list = $derived(owners.length > 1 ? `${owners.slice(0, -1).join(', ')} or ${owners.at(-1)}` : owners[0]);
</script>

<p class="notice" role="note">
  <Icon icon={Lock} size="sm" />
  <span><strong>Only owners can manage {what}.</strong> {#if list}Ask {list} {ask}.{/if}</span>
</p>

<style>
  .notice {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    margin-block-end: var(--space-4);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background: var(--color-fill-subtle);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
  }

  .notice :global(svg) {
    flex: none;
    margin-block-start: var(--space-0-5);
  }

  strong {
    color: var(--color-text);
    font-weight: var(--weight-medium);
  }
</style>
