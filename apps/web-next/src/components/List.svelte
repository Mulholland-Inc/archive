<script>
  let { divided = true, inset = false, label, children } = $props();
</script>

<ul class={['list', { divided, inset }]} aria-label={label}>
  {@render children()}
</ul>

<style>
  .list {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .divided > :global(li + li) {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .inset {
    gap: var(--space-px);
  }
</style>
