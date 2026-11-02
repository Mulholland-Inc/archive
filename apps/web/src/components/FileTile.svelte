<script>
  import { FileText, Image, AudioLines, FileSpreadsheet, Play, Ellipsis } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import { contextmenu } from '../lib/contextmenu.svelte.js';
  import { blobURL } from '../lib/api.js';

  let { file, onclick, selected = false, menu, items } = $props();
  const icons = { image: Image, recording: AudioLines, spreadsheet: FileSpreadsheet, pdf: FileText, document: FileText };
  let thumb = $state('');
  $effect(() => {
    thumb = '';
    if (file.thumb) blobURL(file.thumb).then((u) => (thumb = u)).catch(() => {});
  });
  const bars = [3, 6, 9, 5, 11, 7, 12, 8, 4, 10, 6, 9, 3, 7, 11, 5, 8, 12, 6, 4, 9, 7, 10, 5];
</script>

<article class={['tile', { selected }]} {@attach items && contextmenu(items)}>
  <button type="button" class="hit" {onclick} aria-label={file.name}></button>
  <div class={['thumb', file.kind, `t${file.tone}`]}>
    {#if thumb}
      <img class="real" src={thumb} alt="" loading="lazy" draggable="false" />
    {:else if file.kind === 'image'}
      <span class="photo"><span class="sun"></span><span class="hill"></span><span class="hill back"></span></span>
    {:else if file.kind === 'recording'}
      <span class="wave">
        {#each bars as b, i (i)}<span class={`b b${b}`}></span>{/each}
      </span>
      <span class="play"><Icon icon={Play} size="sm" /></span>
      <span class="duration">{file.duration}</span>
    {:else if file.kind === 'spreadsheet'}
      <span class="sheet">
        {#each { length: 18 } as _, i (i)}<span></span>{/each}
      </span>
    {:else}
      <span class="doc">
        <span class="line h"></span>
        <span class="line"></span>
        <span class="line"></span>
        <span class="line short"></span>
        <span class="line"></span>
        <span class="line short"></span>
      </span>
    {/if}
    <span class="ext">{file.ext}</span>
  </div>
  <div class="meta">
    <span class="kind"><Icon icon={icons[file.kind] ?? FileText} size="sm" /></span>
    <span class="text">
      <span class="name">{file.name}</span>
      <span class="sub">{file.modified}, {file.size}</span>
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
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
    transition: transform var(--duration-slow) var(--ease-out);
  }

  .tile:hover .thumb {
    transform: translateY(calc(var(--space-0-5) * -1));
  }

  .t1 { background: var(--color-thumb-1); }
  .t2 { background: var(--color-thumb-2); }
  .t3 { background: var(--color-thumb-3); }
  .t4 { background: var(--color-thumb-4); }

  .document,
  .pdf,
  .spreadsheet {
    background: var(--color-surface-sunken);
  }

  .photo {
    position: absolute;
    inset: 0;
  }

  .sun {
    position: absolute;
    inset-block-start: var(--tile-sun-inset);
    inset-inline-end: var(--tile-sun-inset);
    width: var(--tile-sun-size);
    aspect-ratio: 1;
    border-radius: var(--radius-full);
    background: var(--color-surface);
    opacity: var(--opacity-soft);
  }

  .hill {
    position: absolute;
    inset-inline: var(--tile-hill-inline);
    inset-block-end: var(--tile-hill-block);
    height: var(--tile-hill-height);
    border-radius: var(--radius-round);
    background: var(--color-text);
    opacity: var(--opacity-faint);
  }

  .hill.back {
    inset-inline: var(--tile-hill-back-inline);
    inset-block-end: var(--tile-hill-back-block);
    opacity: var(--opacity-fainter);
  }

  .doc {
    display: flex;
    flex-direction: column;
    gap: var(--space-1-5);
    width: var(--tile-doc-width);
    height: var(--tile-doc-height);
    margin-block-start: var(--tile-doc-top);
    padding: var(--space-3);
    border-radius: var(--radius-sm) var(--radius-sm) 0 0;
    background: var(--color-surface);
    box-shadow: var(--ring);
  }

  .line {
    height: var(--space-1);
    border-radius: var(--radius-full);
    background: var(--color-fill-strong);
  }

  .line.h {
    width: var(--tile-line-heading);
    height: var(--space-1-5);
    background: var(--color-text-tertiary);
    opacity: var(--opacity-half);
  }

  .line.short {
    width: var(--tile-line-short);
  }

  .sheet {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    width: var(--tile-sheet-size);
    height: var(--tile-sheet-size);
    gap: var(--space-px);
    border-radius: var(--radius-sm);
    overflow: hidden;
    background: var(--color-border);
    box-shadow: var(--ring);
  }

  .sheet span {
    background: var(--color-surface);
  }

  .sheet span:nth-child(-n + 3) {
    background: var(--color-thumb-2);
  }

  .wave {
    display: flex;
    align-items: center;
    gap: var(--space-0-5);
    height: var(--tile-wave-height);
  }

  .b {
    width: var(--space-0-5);
    border-radius: var(--radius-full);
    background: var(--color-text);
    opacity: var(--opacity-dim);
  }

  .b3 { height: var(--tile-bar-3); }
  .b4 { height: var(--tile-bar-4); }
  .b5 { height: var(--tile-bar-5); }
  .b6 { height: var(--tile-bar-6); }
  .b7 { height: var(--tile-bar-7); }
  .b8 { height: var(--tile-bar-8); }
  .b9 { height: var(--tile-bar-9); }
  .b10 { height: var(--tile-bar-10); }
  .b11 { height: var(--tile-bar-11); }
  .b12 { height: var(--tile-bar-12); }

  .play {
    position: absolute;
    inset-block-end: var(--space-2);
    inset-inline-start: var(--space-2);
    display: grid;
    place-items: center;
    width: var(--control-sm);
    height: var(--control-sm);
    border-radius: var(--radius-full);
    background: var(--color-surface);
    color: var(--color-text);
    box-shadow: var(--ring);
  }

  .duration,
  .ext {
    position: absolute;
    padding: var(--space-1);
    border-radius: var(--radius-xs);
    background: var(--color-surface);
    color: var(--color-text-secondary);
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    letter-spacing: var(--tracking-caps);
    line-height: var(--leading-none);
  }

  .duration {
    inset-block-end: var(--space-2);
    inset-inline-end: var(--space-2);
    letter-spacing: var(--tracking-normal);
    font-variant-numeric: tabular-nums;
  }

  .ext {
    inset-block-start: var(--space-2);
    inset-inline-start: var(--space-2);
  }

  .meta {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-1);
  }

  .kind {
    display: inline-flex;
    padding-block-start: var(--space-0-5);
    color: var(--color-text-tertiary);
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
