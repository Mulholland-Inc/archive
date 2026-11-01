<script>
  let { label, stacked = false, children } = $props();
</script>

<div class={['row', { stacked }]}>
  <span class="label">{label}</span>
  <div class="control">{@render children()}</div>
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    min-height: var(--list-row);
    padding-block: var(--space-1-5);
  }

  .row + :global(.row) {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .stacked {
    flex-direction: column;
    align-items: stretch;
    gap: var(--space-2);
  }

  .label {
    font-size: var(--text-md);
    color: var(--color-text);
  }

  .control {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
</style>
