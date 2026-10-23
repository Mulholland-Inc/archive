<script lang="ts">
  import type { Snippet } from 'svelte';
  let { children }: { children: Snippet } = $props();
</script>

<div class="card">{@render children()}</div>

<style>
  .card {
    width: calc(var(--menu-width) * 1.4);
    padding: var(--space-3);
    border-radius: var(--menu-radius);
    background: var(--popover);
    color: var(--popover-foreground);
    box-shadow: var(--shadow-popover);
  }
</style>
