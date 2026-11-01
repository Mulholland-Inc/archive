<script>
  let { title, children, column = false } = $props();
</script>

<div class="block">
  <div class="title">{title}</div>
  <div class={['demo', { column }]}>{@render children()}</div>
</div>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
  }

  .title {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
  }

  .demo {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
    padding: var(--space-5);
    border-radius: var(--radius-xl);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .column {
    flex-direction: column;
    align-items: stretch;
  }
</style>
