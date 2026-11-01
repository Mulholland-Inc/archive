<script>
  let { token, family = 'sans', label, children } = $props();
</script>

<div class="row">
  <code>{label ?? `--${token}`}</code>
  <span class={['sample', token, family]}>{@render children()}</span>
</div>

<style>
  .row {
    display: grid;
    grid-template-columns: var(--col-md) 1fr;
    align-items: baseline;
    gap: var(--space-4);
    padding-block: var(--space-3);
    min-width: 0;
  }

  .row + :global(.row) {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  code {
    color: var(--color-text-tertiary);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    letter-spacing: var(--tracking-normal);
  }

  .sample {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: var(--leading-tight);
  }

  .serif { font-family: var(--font-serif); font-weight: var(--weight-serif); letter-spacing: var(--tracking-serif); }

  .text-2xs { font-size: var(--text-2xs); }
  .text-xs { font-size: var(--text-xs); }
  .text-sm { font-size: var(--text-sm); }
  .text-md { font-size: var(--text-md); }
  .text-base { font-size: var(--text-base); }
  .text-lg { font-size: var(--text-lg); }
  .text-xl { font-size: var(--text-xl); }
  .text-2xl { font-size: var(--text-2xl); }
  .text-3xl { font-size: var(--text-3xl); letter-spacing: var(--tracking-tight); }
  .text-4xl { font-size: var(--text-4xl); letter-spacing: var(--tracking-tight); }
</style>
