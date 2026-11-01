<script>
  let {
    as = 'div',
    direction = 'column',
    gap = '4',
    align = 'stretch',
    justify = 'start',
    wrap = false,
    grow = false,
    full = false,
    children,
    ...rest
  } = $props();
</script>

<svelte:element this={as} class={['stack', direction, `g${gap}`, `a-${align}`, `j-${justify}`, { wrap, grow, full }]} {...rest}>
  {@render children?.()}
</svelte:element>

<style>
  .stack {
    display: flex;
    min-width: 0;
  }

  .column { flex-direction: column; }
  .row { flex-direction: row; }
  .wrap { flex-wrap: wrap; }
  .grow { flex: 1; min-height: 0; }
  .full { width: 100%; }

  .g0 { gap: var(--space-0); }
  .g0-5 { gap: var(--space-0-5); }
  .g1 { gap: var(--space-1); }
  .g1-5 { gap: var(--space-1-5); }
  .g2 { gap: var(--space-2); }
  .g3 { gap: var(--space-3); }
  .g4 { gap: var(--space-4); }
  .g5 { gap: var(--space-5); }
  .g6 { gap: var(--space-6); }
  .g8 { gap: var(--space-8); }
  .g10 { gap: var(--space-10); }
  .g12 { gap: var(--space-12); }
  .g16 { gap: var(--space-16); }

  .a-stretch { align-items: stretch; }
  .a-start { align-items: flex-start; }
  .a-center { align-items: center; }
  .a-end { align-items: flex-end; }
  .a-baseline { align-items: baseline; }

  .j-start { justify-content: flex-start; }
  .j-center { justify-content: center; }
  .j-end { justify-content: flex-end; }
  .j-between { justify-content: space-between; }
</style>
