<script lang="ts">
  import type { Snippet } from 'svelte';
  let { start, end }: { start: Snippet; end?: Snippet } = $props();
</script>

<div class="toolbar">
  <div class="group">{@render start()}</div>
  {#if end}<div class="group">{@render end()}</div>{/if}
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-6) 0;
  }
  .group {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
</style>
