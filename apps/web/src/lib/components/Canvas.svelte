<script lang="ts">
  import type { Snippet } from 'svelte';
  let { children }: { children: Snippet } = $props();
</script>

<div class="canvas">{@render children()}</div>

<style>
  .canvas {
    min-height: 100vh;
    min-height: 100dvh;
    padding: var(--space-12) var(--space-10) calc(var(--space-20) + var(--space-4));
    background: var(--surface);
  }
</style>
