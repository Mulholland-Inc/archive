<script>
  let { aside, children, side = 'end', size = 'md', sticky = false } = $props();
</script>

<div class={['split', side, size, { sticky }]}>
  {#if side === 'start'}<aside class="aside">{@render aside()}</aside>{/if}
  <div class="primary">{@render children()}</div>
  {#if side === 'end'}<aside class="aside">{@render aside()}</aside>{/if}
</div>

<style>
  .split {
    display: grid;
    gap: var(--space-8);
    align-items: start;
    min-width: 0;
  }

  .end { grid-template-columns: minmax(0, 1fr) var(--col-lg); }
  .end.lg { grid-template-columns: minmax(0, 1fr) var(--popover-width); }
  .start { grid-template-columns: var(--settings-nav) minmax(0, 1fr); }

  .primary,
  .aside {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    min-width: 0;
  }

  .sticky .aside {
    position: sticky;
    inset-block-start: var(--space-8);
  }

  @media (max-width: 64rem) {
    .split,
    .end.lg {
      grid-template-columns: minmax(0, 1fr);
    }

    .sticky .aside {
      position: static;
    }
  }
</style>
