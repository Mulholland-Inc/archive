<script>
  import { contextmenu } from '../lib/contextmenu.svelte.js';
  import { blobURL } from '../lib/api.js';
  import { fileArt } from '../lib/live.js';

  let { file, onclick, selected = false, menu, items } = $props();
  let thumb = $state('');
  $effect(() => {
    thumb = '';
    if (file.thumb) blobURL(file.thumb).then((u) => (thumb = u)).catch(() => {});
  });
</script>

<article class={['tile', { selected }]} {@attach items && contextmenu(items)}>
  <button type="button" class="hit" {onclick} aria-label={file.name}></button>
  <div class="thumb">
    {#if thumb}
      <img class="real" src={thumb} alt="" loading="lazy" draggable="false" />
    {:else}
      <img class="art" src={fileArt(file.name, file.kind)} alt="" draggable="false" />
    {/if}
    {#if file.duration}<span class="duration">{file.duration}</span>{/if}
  </div>
  <div class="meta">
    <span class="text">
      <span class="name">{file.name}</span>
      <span class="sub">{file.sub ?? `${file.modified}, ${file.size}`}</span>
    </span>
    {#if menu}<span class="menu">{@render menu()}</span>{/if}
  </div>
</article>

<style>
  .real {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .tile {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-2);
    border-radius: var(--radius-xl);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .tile:hover,
  .selected,
  .tile:global([data-contextmenu]) {
    background: var(--color-fill-subtle);
  }

  .hit {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    z-index: var(--z-raised);
  }

  .menu {
    position: relative;
    z-index: var(--z-raised);
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .tile:hover .menu,
  .menu:focus-within,
  .menu:has(:global([aria-expanded='true'])) {
    opacity: 1;
  }

  .thumb {
    position: relative;
    display: grid;
    place-items: center;
    aspect-ratio: var(--thumb-ratio);
    border-radius: var(--radius-lg);
    overflow: hidden;
    background: var(--color-surface-sunken);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
    transition: transform var(--duration-slow) var(--ease-out);
  }

  .tile:hover .thumb {
    transform: translateY(calc(var(--space-0-5) * -1));
  }

  .art {
    height: 64%;
    aspect-ratio: 1;
  }

  .duration {
    position: absolute;
    inset-block-end: var(--space-2);
    inset-inline-end: var(--space-2);
    padding: var(--space-1);
    border-radius: var(--radius-xs);
    background: var(--color-surface);
    color: var(--color-text-secondary);
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    line-height: var(--leading-none);
    font-variant-numeric: tabular-nums;
  }

  .meta {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-1);
  }

  .text {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .sub {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }
</style>
