<script lang="ts">
  import type { Snippet } from 'svelte';
  let { min = 'md', children }: { min?: 'xs' | 'tile' | 'sm' | 'md' | 'lg'; children: Snippet } = $props();
</script>

<div class="grid" data-min={min}>{@render children()}</div>

<style>
  .grid {
    display: grid;
    gap: var(--space-4);
    grid-template-columns: repeat(auto-fill, minmax(var(--min), 1fr));
    align-items: start;
  }
  [data-min='xs'] { --min: 8rem; }
  [data-min='tile'] { --min: 9.5rem; }
  [data-min='sm'] { --min: 12rem; }
  [data-min='md'] { --min: 18rem; }
  [data-min='lg'] { --min: 24rem; }
</style>
