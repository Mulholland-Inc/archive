<script>
  let { children } = $props();
</script>

<span class="chip">{@render children()}</span>

<style>
  .chip {
    display: inline-flex;
    flex: none;
    align-items: center;
    padding: var(--space-1);
    border-radius: var(--radius-xs);
    background: var(--fill);
    color: var(--fg-muted);
    font-size: var(--text-meta);
    line-height: var(--leading-none);
    white-space: nowrap;
  }
</style>
