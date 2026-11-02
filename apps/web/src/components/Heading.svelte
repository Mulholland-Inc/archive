<script>
  // A heading at any level. size picks the type step (sm text-md, md text-base, lg text-xl, xl text-2xl,
  // 2xl text-3xl, 3xl text-4xl); serif headings are the archive's voice, sans ones label sections.
  // leading="none" and tracking="tight" set a display heading, leading="inherit" keeps the line height
  // around it; wrap="anywhere" lets a long name break, wrap="normal" breaks it as text does,
  // instead of balancing its lines; grow fills the row it sits in; step steps it down to lg in
  // a container under 32rem.
  let { level = 2, size = 'lg', serif = true, leading = 'tight', tracking, wrap = 'balance', grow = false, step = false, children, ...rest } = $props();
</script>

<svelte:element this={`h${level}`} class={['heading', `s-${size}`, `l-${leading}`, `w-${wrap}`, tracking && `t-${tracking}`, { serif, grow, step }]} {...rest}>
  {@render children()}
</svelte:element>

<style>
  .heading {
    color: var(--color-text);
    font-family: var(--font-sans);
    font-weight: var(--weight-semibold);
    letter-spacing: var(--tracking-snug);
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
  .s-2xl { font-size: var(--text-3xl); }
  .s-3xl { font-size: var(--text-4xl); }

  .serif.s-lg { font-size: calc(var(--text-xl) + var(--space-0-5)); }

  .l-tight { line-height: var(--leading-tight); }
  .l-none { line-height: var(--leading-none); }

  .t-tight { letter-spacing: var(--tracking-tight); }

  .w-balance { text-wrap: balance; }
  .w-anywhere { overflow-wrap: anywhere; }

  .grow { flex: 1; }

  @container (max-width: 32rem) {
    .step {
      font-size: var(--text-xl);
    }
  }
</style>
