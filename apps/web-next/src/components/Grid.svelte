<script>
  let { min = 'card', gap = '4', columns, children, ...rest } = $props();
</script>

<div class={['grid', `m-${min}`, `g${gap}`, columns && `c${columns}`]} {...rest}>
  {@render children()}
</div>

<style>
  .grid {
    display: grid;
    width: 100%;
    min-width: 0;
  }

  .m-tile { grid-template-columns: repeat(auto-fill, minmax(var(--tile-min), 1fr)); }
  .m-card { grid-template-columns: repeat(auto-fill, minmax(var(--card-min), 1fr)); }
  .c2 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .c3 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .c4 { grid-template-columns: repeat(4, minmax(0, 1fr)); }

  .g2 { gap: var(--space-2); }
  .g3 { gap: var(--space-3); }
  .g4 { gap: var(--space-4); }
  .g6 { gap: var(--space-6); }
  .g8 { gap: var(--space-8); }
</style>
