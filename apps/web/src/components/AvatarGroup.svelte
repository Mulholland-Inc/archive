<script>
  import Avatar from './Avatar.svelte';
  import { seedOf } from '../lib/app.svelte.js';

  let { people = [], max = 4, size = 'sm' } = $props();
  const shown = $derived(people.slice(0, max));
  const extra = $derived(people.length - shown.length);
</script>

<span class={['group', size]} aria-label={people.map((p) => p.name).join(', ')} role="img">
  {#each shown as p (seedOf(p))}
    <Avatar name={p.name} seed={seedOf(p)} {size} />
  {/each}
  {#if extra > 0}
    <span class="more">+{extra}</span>
  {/if}
</span>

<style>
  .group {
    display: inline-flex;
    align-items: center;
    flex: none;
    padding-inline-start: var(--space-1);
  }

  .group > :global(*) {
    margin-inline-start: calc(var(--space-1) * -1);
  }

  .more {
    display: inline-grid;
    place-items: center;
    height: var(--avatar-sm);
    min-width: var(--avatar-sm);
    padding: var(--space-1);
    border-radius: var(--radius-full);
    background: var(--color-fill-strong);
    color: var(--color-text-secondary);
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
  }

  .xs .more {
    height: var(--avatar-xs);
    min-width: var(--avatar-xs);
  }

  .md .more {
    height: var(--avatar-md);
    min-width: var(--avatar-md);
  }
</style>
