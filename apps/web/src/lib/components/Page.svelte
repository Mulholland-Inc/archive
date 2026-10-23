<script lang="ts">
  import type { Snippet } from 'svelte';
  let { width = 'default', children }: { width?: 'default' | 'full' | 'narrow'; children: Snippet } = $props();
</script>

<div class="scroll">
  <div class="page" data-width={width}>{@render children()}</div>
</div>

<style>
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    max-width: var(--page-width);
    padding: var(--space-6) var(--space-8) var(--space-16);
    animation: page-in var(--duration-slow) var(--ease-out);
  }
  [data-width='full'] {
    max-width: none;
  }
  [data-width='narrow'] {
    max-width: calc(var(--form-width) + var(--space-16));
  }
  @keyframes page-in {
    from {
      opacity: 0;
      transform: translateY(var(--nudge));
    }
  }
  @media (max-width: 40rem) {
    .page {
      gap: var(--space-5);
      padding: var(--space-4) var(--space-4) var(--space-12);
    }
  }
</style>
