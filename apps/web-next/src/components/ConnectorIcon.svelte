<script>
  import { Users, Plug } from '@lucide/svelte';
  import { brandFor } from '../lib/brands.js';

  let { id, size = 'md', label } = $props();
  const brand = $derived(brandFor(id));
  const Glyph = $derived(id === 'scim' ? Users : Plug);
</script>

<span class={['tile', size]} role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : 'true'}>
  {#if brand}
    <svg viewBox={brand[0]}>{@html brand[1]}</svg>
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
