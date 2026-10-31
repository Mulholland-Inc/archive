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
    width: var(--control-xl);
    height: var(--control-xl);
    border-radius: var(--radius-lg);
    background: var(--color-surface-sunken);
    box-shadow: var(--ring-subtle);
    color: var(--color-text);
  }

  .sm {
    width: var(--control-md);
    height: var(--control-md);
    border-radius: var(--radius-md);
  }

  .xs {
    width: var(--icon-lg);
    height: var(--icon-lg);
    border-radius: var(--radius-xs);
    background: none;
    box-shadow: none;
  }

  .lg {
    width: var(--control-2xl);
    height: var(--control-2xl);
  }

  svg,
  .glyph {
    display: block;
    width: 50%;
    height: 50%;
  }

  .xs svg,
  .xs .glyph {
    width: 80%;
    height: 80%;
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
