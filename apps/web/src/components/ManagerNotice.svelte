<script>
  import { Lock } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import { atLeast, ws, members } from '../lib/app.svelte.js';

  members.use();

  let { what = 'this' } = $props();
  const names = $derived(ws().members.filter((m) => atLeast('manager', m.role) && m.status === 'active').map((m) => m.name));
  const list = $derived(names.length > 1 ? `${names.slice(0, -1).join(', ')} or ${names.at(-1)}` : names[0]);
</script>

<p class="notice" role="note">
  <Icon icon={Lock} size="sm" />
  <span><strong>Only managers, engineers and the owner can manage {what}.</strong> {#if list}Ask {list} to connect or change one.{/if}</span>
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
