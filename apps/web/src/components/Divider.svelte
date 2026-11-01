<script>
  let { vertical = false, label, spaced = false } = $props();
</script>

{#if label}
  <div class={['labelled', { spaced }]} role="separator">
    <span>{label}</span>
  </div>
{:else}
  <hr class={['divider', { vertical, spaced }]} aria-orientation={vertical ? 'vertical' : 'horizontal'} />
{/if}

<style>
  .divider {
    flex: none;
    align-self: stretch;
    border: 0;
    height: var(--border-width);
    background: var(--color-border-subtle);
  }

  .vertical {
    width: var(--border-width);
    height: auto;
  }

  .spaced {
    margin-block: var(--space-2);
  }

  .labelled {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .labelled::before,
  .labelled::after {
    content: '';
    flex: 1;
    height: var(--border-width);
    background: var(--color-border-subtle);
  }
</style>
