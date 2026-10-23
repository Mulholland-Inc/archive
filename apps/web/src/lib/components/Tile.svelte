<script lang="ts">
  import Icon from './Icon.svelte';
  let { icon, color = 5, size = 'md' }: { icon: string; color?: number; size?: 'md' | 'lg' } = $props();
</script>

<span class="tile" data-color={color} data-size={size}><Icon name={icon} size="sm" /></span>

<style>
  .tile {
    flex: none;
    display: inline-grid;
    place-items: center;
    width: calc(var(--icon-size) + var(--space-1-5));
    height: calc(var(--icon-size) + var(--space-1-5));
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--tile) 42%, var(--card));
    color: color-mix(in srgb, var(--tile) 70%, var(--ink));
  }
  [data-size='lg'] {
    width: calc(var(--icon-size) * 2);
    height: calc(var(--icon-size) * 2);
    border-radius: var(--radius-md);
  }
  [data-color='1'] { --tile: var(--label-1); }
  [data-color='2'] { --tile: var(--label-2); }
  [data-color='3'] { --tile: var(--label-3); }
  [data-color='4'] { --tile: var(--label-4); }
  [data-color='5'] { --tile: var(--label-5); }
</style>
