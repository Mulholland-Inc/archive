<script>
  // A few figures in two columns, each a small label over its value, for a details sidebar.
  let { items = [] } = $props();
</script>

<dl class="stats">
  {#each items as item (item.label)}
    <div><dt>{item.label}</dt><dd>{item.value}</dd></div>
  {/each}
</dl>

<style>
  .stats {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4) var(--space-3);
    padding: var(--space-1) var(--space-1-5);
  }

  dt {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
  }

  dd {
    font-size: var(--text-lg);
    font-weight: var(--weight-medium);
    font-variant-numeric: tabular-nums;
  }
</style>
