<script>
  let { icon: Glyph, size = 'md', label } = $props();
</script>

<span class={['icon', size]} role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : 'true'}>
  <Glyph />
</span>

<style>
  .icon {
    display: inline-flex;
    flex: none;
    width: var(--icon-md);
    height: var(--icon-md);
  }

  .icon :global(svg) {
    width: 100%;
    height: 100%;
    stroke-width: var(--icon-stroke);
  }

  .xs {
    width: var(--icon-xs);
    height: var(--icon-xs);
  }

  .sm {
    width: var(--icon-sm);
    height: var(--icon-sm);
  }

  .lg {
    width: var(--icon-lg);
    height: var(--icon-lg);
  }

  .xl {
    width: var(--icon-xl);
    height: var(--icon-xl);
  }

  .xxl {
    width: var(--icon-2xl);
    height: var(--icon-2xl);
  }
</style>
