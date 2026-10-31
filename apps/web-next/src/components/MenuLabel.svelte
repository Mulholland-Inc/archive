<script>
  let { children } = $props();
</script>

<div class="label">{@render children()}</div>

<style>
  .label {
    padding: var(--space-1-5);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
