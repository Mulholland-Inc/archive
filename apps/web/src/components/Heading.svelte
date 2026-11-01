<script>
  let { level = 2, size = 'lg', serif = true, children, ...rest } = $props();
</script>

<svelte:element this={`h${level}`} class={['heading', `s-${size}`, { serif }]} {...rest}>
  {@render children()}
</svelte:element>

<style>
  .heading {
    color: var(--color-text);
    font-family: var(--font-sans);
    font-weight: var(--weight-semibold);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-snug);
    text-wrap: balance;
    min-width: 0;
  }

  .serif {
    font-family: var(--font-serif);
    font-weight: var(--weight-serif);
    letter-spacing: var(--tracking-serif);
  }

  .s-sm { font-size: var(--text-md); }
  .s-md { font-size: var(--text-base); }
  .s-lg { font-size: var(--text-xl); }
  .s-xl { font-size: var(--text-2xl); }
  .s-2xl { font-size: var(--text-3xl); line-height: var(--leading-none); letter-spacing: var(--tracking-tight); }
  .s-3xl { font-size: var(--text-4xl); line-height: var(--leading-none); letter-spacing: var(--tracking-tight); }

  .serif.s-lg { font-size: calc(var(--text-xl) + var(--space-0-5)); }
</style>
