<script>
  let { tone = 'neutral', dot = false, pulse = false, children } = $props();
</script>

<span class={['badge', tone]}>
  {#if dot}<span class={['dot', { pulse }]}></span>{/if}
  {@render children()}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    flex: none;
    width: fit-content;
    height: var(--avatar-xs);
    padding: var(--space-1);
    border-radius: var(--radius-sm);
    background: var(--color-fill);
    color: var(--color-text-secondary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    line-height: var(--leading-none);
    white-space: nowrap;
  }

  .dot {
    width: var(--space-1-5);
    height: var(--space-1-5);
    border-radius: var(--radius-full);
    background: currentColor;
  }

  .pulse {
    animation: pulse calc(var(--duration-slower) * 4) var(--ease-in-out) infinite;
  }

  .accent {
    background: var(--color-accent-subtle);
    color: var(--color-accent-text);
  }

  .success {
    background: var(--color-success-subtle);
    color: var(--color-success);
  }

  .warning {
    background: var(--color-warning-subtle);
    color: var(--color-warning);
  }

  .danger {
    background: var(--color-danger-subtle);
    color: var(--color-danger);
  }

  .outline {
    background: none;
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  @keyframes pulse {
    50% {
      opacity: 0.35;
    }
  }
</style>
