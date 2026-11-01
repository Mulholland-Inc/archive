<script>
  import TypeArt from './TypeArt.svelte';

  let { type, onclick, selected = false } = $props();
</script>

<article class={['tile', { selected }]}>
  <button type="button" class="hit" {onclick} aria-label={type.name}></button>
  <div class="thumb">
    <TypeArt icon={type.icon} />
  </div>
  <div class="meta">
    <span class="text">
      <span class="name">{type.name}</span>
      <span class="sub">{type.rows.toLocaleString('en-US')} rows, {type.columns.length} columns</span>
    </span>
  </div>
</article>

<style>
  .tile {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-2);
    border-radius: var(--radius-xl);
    box-shadow: var(--ring);
    transition: background-color var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out);
  }

  .tile:hover,
  .tile:focus-within,
  .selected {
    background: var(--color-fill-subtle);
    box-shadow: var(--ring-strong);
  }

  .hit {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    z-index: var(--z-raised);
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
