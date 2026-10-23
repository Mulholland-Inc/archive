<script lang="ts">
  import Icon from './Icon.svelte';
  let { name, onclick }: { name: string; onclick?: () => void } = $props();
</script>

<button class="switcher" {onclick}>
  <span class="logo"><Icon name="brand" /></span>
  <span class="name">{name}</span>
  <Icon name="chevron-down" size="sm" />
</button>

<style>
  .switcher {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    height: var(--control-height);
    padding: 0 var(--space-1-5) 0 var(--space-1);
    margin-left: calc(var(--space-1) * -1);
    border-radius: var(--radius);
    font-size: var(--text-md);
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .switcher:hover {
    background: var(--hover);
  }
  .switcher:active {
    transform: scale(var(--press-scale));
  }
  .logo {
    display: grid;
    place-items: center;
    width: calc(var(--icon-size) + var(--space-1));
    height: calc(var(--icon-size) + var(--space-1));
    border-radius: var(--radius-sm);
    background: var(--foreground);
    color: var(--highlight);
    --glyph-cut: var(--foreground);
  }
  .logo :global(svg) {
    width: var(--icon-size-sm);
    height: var(--icon-size-sm);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
