<script>
  import { Users, Plug } from '@lucide/svelte';
  import { brandFor } from '../lib/brands.js';

  let { id, size = 'md', label } = $props();
  const brand = $derived(brandFor(id));
  // A logo's gradients and clips point at ids inside it. With the same logo drawn twice, and one copy hidden in a
  // closed menu, the browser resolves those ids to the hidden copy and paints nothing, so each icon gets its own.
  const uid = $props.id();
  const markup = $derived(brand?.[1].replace(/(id="|url\(#|href="#)([^")]+)/g, `$1$2-${uid}`));
  const Glyph = $derived(id === 'scim' ? Users : Plug);
</script>

<span class={['tile', size]} role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : 'true'}>
  {#if brand}
    <svg viewBox={brand[0]}>{@html markup}</svg>
  {:else}
    <span class="glyph"><Glyph /></span>
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

</style>
