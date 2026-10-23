<script lang="ts">
  import type { Snippet } from 'svelte';
  let { open = true, children }: { open?: boolean; children: Snippet } = $props();
</script>

<div class="collapse" data-open={open || undefined}>
  <div class="inner">{@render children()}</div>
</div>

<style>
  .collapse {
    display: grid;
    grid-template-rows: 0fr;
    opacity: 0;
    transition:
      grid-template-rows var(--duration-slow) var(--ease-out),
      opacity var(--duration-normal) var(--ease-out);
  }
  .collapse[data-open] {
    grid-template-rows: 1fr;
    opacity: 1;
  }
  .inner {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-1-5);
    min-height: 0;
    overflow: hidden;
    padding: var(--space-0-5) var(--space-0-5) var(--space-0-5) calc(var(--space-6) - var(--space-0-5));
    margin: calc(var(--space-0-5) * -1);
  }
</style>
