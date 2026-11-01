<script>
  let {
    as = 'span',
    size = 'md',
    tone = 'default',
    weight = 'regular',
    family = 'sans',
    truncate = false,
    tabular = false,
    children,
    ...rest
  } = $props();
</script>

<svelte:element this={as} class={['text', `s-${size}`, `t-${tone}`, `w-${weight}`, `f-${family}`, { truncate, tabular }]} {...rest}>
  {@render children()}
</svelte:element>

<style>
  .text {
    min-width: 0;
  }

  .s-2xs { font-size: var(--text-2xs); }
  .s-xs { font-size: var(--text-xs); }
  .s-sm { font-size: var(--text-sm); }
  .s-md { font-size: var(--text-md); }
  .s-base { font-size: var(--text-base); }
  .s-lg { font-size: var(--text-lg); }

  .t-default { color: var(--color-text); }
  .t-secondary { color: var(--color-text-secondary); }
  .t-tertiary { color: var(--color-text-tertiary); }
  .t-accent { color: var(--color-accent-text); }
  .t-success { color: var(--color-success); }
  .t-warning { color: var(--color-warning); }
  .t-danger { color: var(--color-danger); }

  .w-regular { font-weight: var(--weight-regular); }
  .w-medium { font-weight: var(--weight-medium); }
  .w-semibold { font-weight: var(--weight-semibold); }

  .f-sans { font-family: var(--font-sans); }
  .f-serif { font-family: var(--font-serif); letter-spacing: var(--tracking-normal); }

  .truncate {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tabular {
    font-variant-numeric: tabular-nums;
  }
</style>
