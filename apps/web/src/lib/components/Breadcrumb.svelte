<script lang="ts">
  import Icon from './Icon.svelte';
  import Tile from './Tile.svelte';
  let {
    items
  }: { items: { label: string; tile?: { icon: string; color: number }; onclick?: () => void }[] } = $props();
</script>

<ol class="crumbs">
  {#each items as item, i (item.label)}
    {#if i > 0}<li class="sep" aria-hidden="true"><Icon name="chevron-right" size="sm" /></li>{/if}
    <li>
      <button class="crumb" aria-current={i === items.length - 1 ? 'page' : undefined} onclick={item.onclick}>
        {#if item.tile}<Tile icon={item.tile.icon} color={item.tile.color} />{/if}
        {item.label}
      </button>
    </li>
  {/each}
</ol>

<style>
  .crumbs {
    display: flex;
    align-items: center;
    gap: var(--space-0-5);
    list-style: none;
    min-width: 0;
    font-size: var(--text-md);
  }
  li {
    display: flex;
    min-width: 0;
  }
  .sep {
    color: var(--foreground);
    padding: 0 var(--space-1);
  }
  .crumb {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    height: var(--control-height);
    padding: 0 var(--space-1-5);
    border-radius: var(--radius);
    white-space: nowrap;
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  .crumb:hover {
    background: var(--hover);
  }
  li:last-child {
    min-width: 0;
  }
  .crumb {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  @media (max-width: 40rem) {
    li:not(:last-child) {
      display: none;
    }
  }
</style>
