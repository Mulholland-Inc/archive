<script lang="ts">
  import Icon from './Icon.svelte';
  import Tile from './Tile.svelte';
  let {
    label,
    icon,
    glyph,
    tile,
    active = false,
    expanded,
    trailing,
    onclick
  }: {
    label: string;
    icon?: string;
    glyph?: string;
    tile?: { icon: string; color: number };
    active?: boolean;
    expanded?: boolean;
    trailing?: boolean;
    onclick?: () => void;
  } = $props();
</script>

<button class="item" data-active={active || undefined} data-plain={tile ? true : undefined} aria-current={active ? 'page' : undefined} aria-expanded={expanded} {onclick}>
  {#if tile}<Tile icon={tile.icon} color={tile.color} />{:else if glyph}<Icon name={glyph} />{:else if icon}<Icon name={icon} />{/if}
  <span class="label">{label}</span>
  {#if expanded !== undefined || trailing}
    <span class="chev" data-rotates={expanded !== undefined || undefined} data-open={expanded || undefined}><Icon name={expanded !== undefined ? 'chevron-down' : 'chevron-right'} size="sm" /></span>
  {/if}
</button>

<style>
  .item {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    max-width: 100%;
    height: var(--sidebar-item-height);
    padding: 0 var(--space-2);
    border-radius: var(--radius);
    background: var(--sidebar-item-background);
    font-size: var(--text-base);
    text-align: start;
    white-space: nowrap;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      box-shadow var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .item:hover {
    background: var(--tint-3);
  }
  .item:active {
    transform: scale(var(--press-scale));
  }
  [data-plain] {
    background: transparent;
    padding-left: var(--space-1);
  }
  [data-active],
  [data-active]:hover {
    background: var(--selected);
    color: var(--selected-foreground);
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .chev {
    display: grid;
    color: var(--foreground);
    transition: transform var(--duration-normal) var(--ease-spring);
  }
  .chev[data-rotates]:not([data-open]) {
    transform: rotate(-90deg);
  }
</style>
