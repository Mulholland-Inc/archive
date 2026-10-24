<script lang="ts">
  import Icon from './Icon.svelte';
  import Tile from './Tile.svelte';
  import Count from './Count.svelte';
  import { intercept } from '../router.svelte';
  let {
    label,
    href,
    icon,
    tile,
    active = false,
    external = false,
    expanded,
    count,
    onclick
  }: {
    label: string;
    href?: string;
    icon?: string;
    tile?: { icon: string; color: number };
    active?: boolean;
    /** a page the dashboard does not route, such as a module's: the browser loads it */
    external?: boolean;
    expanded?: boolean;
    count?: number;
    onclick?: () => void;
  } = $props();
</script>

{#snippet content()}
  {#if tile}<Tile icon={tile.icon} color={tile.color} />{:else if icon}<Icon name={icon} />{/if}
  <span class="label">{label}</span>
  {#if count}<Count value={count} />{/if}
  {#if expanded !== undefined}
    <span class="chev" data-open={expanded || undefined}><Icon name="chevron-down" size="sm" /></span>
  {/if}
{/snippet}

{#if href}
  <a class="item" {href} data-active={active || undefined} aria-current={active ? 'page' : undefined} onclick={(e) => !external && intercept(e, href)}>
    {@render content()}
  </a>
{:else}
  <button class="item" data-active={active || undefined} aria-expanded={expanded} {onclick}>{@render content()}</button>
{/if}

<style>
  .item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    height: var(--sidebar-item-height);
    padding: 0 var(--space-2);
    border-radius: var(--radius);
    color: var(--foreground);
    text-decoration: none;
    font-size: var(--text-base);
    text-align: start;
    white-space: nowrap;
    user-select: none;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .item:hover {
    background: var(--hover);
  }
  .item:active {
    transform: scale(var(--press-scale-subtle));
  }
  [data-active],
  [data-active]:hover {
    background: var(--selected);
    color: var(--selected-foreground);
    box-shadow: 0 0 0 var(--border-width) var(--tint-2);
  }
  .item > :global(.icon) {
    color: var(--muted-foreground);
  }
  [data-active] > :global(.icon) {
    color: var(--foreground);
  }
  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .chev {
    display: grid;
    color: var(--muted-foreground);
    transition: transform var(--duration-normal) var(--ease-spring);
  }
  .chev:not([data-open]) {
    transform: rotate(-90deg);
  }
</style>
