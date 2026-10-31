<script>
  import { Mail, Cloud, Hash, Plug } from '@lucide/svelte';
  import { brands, isDark } from '../lib/brands.js';

  let { id, size = 'md', label } = $props();
  const glyphs = { mail: Mail, cloud: Cloud, hash: Hash };
  const brand = $derived(brands[id]);
  const Glyph = $derived(glyphs[brand?.glyph] ?? Plug);
  const dark = $derived(brand ? isDark(brand.hex) : false);
</script>

<span class={['tile', size, { dark }]} role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : 'true'}>
  {#if brand?.path}
    <svg viewBox="0 0 24 24"><path d={brand.path} fill={`#${brand.hex}`} /></svg>
  {:else}
    <span class="glyph"><Glyph color={brand ? `#${brand.hex}` : 'currentColor'} /></span>
  {/if}
</span>

<style>
  .tile {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--icon-2xl);
    height: var(--icon-2xl);
    color: var(--color-text);
  }

  .xs {
    width: var(--icon-md);
    height: var(--icon-md);
  }

  .sm {
    width: var(--icon-lg);
    height: var(--icon-lg);
  }

  .lg {
    width: var(--control-xl);
    height: var(--control-xl);
  }

  svg,
  .glyph {
    display: block;
    width: 100%;
    height: 100%;
  }

  .glyph :global(svg) {
    width: 100%;
    height: 100%;
    stroke-width: var(--icon-stroke);
  }

  :global([data-theme='dark']) .dark path {
    fill: var(--color-text);
  }

  :global([data-theme='dark']) .dark .glyph :global(svg) {
    stroke: var(--color-text);
  }
</style>
