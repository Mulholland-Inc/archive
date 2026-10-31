<script>
  let { keys = '', subtle = false } = $props();
  const parts = $derived(Array.isArray(keys) ? keys : keys.split(' '));
</script>

<span class={['kbd', { subtle }]}>
  {#each parts as key, i (i)}
    <kbd>{key}</kbd>
  {/each}
</span>

<style>
  .kbd {
    display: inline-flex;
    gap: var(--space-0-5);
    flex: none;
  }

  kbd {
    display: inline-grid;
    place-items: center;
    min-width: var(--avatar-xs);
    height: var(--avatar-xs);
    padding: var(--space-0-5);
    border-radius: var(--radius-xs);
    background: var(--color-fill);
    color: var(--color-text-secondary);
    font-family: var(--font-sans);
    font-size: var(--text-2xs);
    font-weight: var(--weight-medium);
    line-height: var(--leading-none);
  }

  .subtle kbd {
    background: none;
    color: var(--color-text-tertiary);
    min-width: 0;
    padding: var(--space-0);
  }
</style>
