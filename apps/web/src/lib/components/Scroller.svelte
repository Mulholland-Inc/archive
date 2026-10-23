<script lang="ts">
  import type { Snippet } from 'svelte';
  let { axis = 'both', children }: { axis?: 'both' | 'x' | 'y'; children: Snippet } = $props();
</script>

<div class="scroller" data-axis={axis}>{@render children()}</div>

<style>
  .scroller {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--space-4) var(--space-6) var(--space-12);
  }
  [data-axis='x'] {
    overflow-y: hidden;
  }
  [data-axis='y'] {
    overflow-x: hidden;
  }
</style>
