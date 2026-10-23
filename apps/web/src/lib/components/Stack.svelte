<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    gap = '4',
    align = 'stretch',
    children
  }: { gap?: string; align?: 'stretch' | 'start' | 'center' | 'end'; children: Snippet } = $props();
</script>

<div class="stack" data-gap={gap} data-align={align}>{@render children()}</div>

<style>
  .stack {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  [data-align='start'] { align-items: flex-start; }
  [data-align='center'] { align-items: center; }
  [data-align='end'] { align-items: flex-end; }
  [data-gap='0'] { gap: 0; }
  [data-gap='1'] { gap: var(--space-1); }
  [data-gap='2'] { gap: var(--space-2); }
  [data-gap='3'] { gap: var(--space-3); }
  [data-gap='4'] { gap: var(--space-4); }
  [data-gap='5'] { gap: var(--space-5); }
  [data-gap='6'] { gap: var(--space-6); }
  [data-gap='8'] { gap: var(--space-8); }
  [data-gap='10'] { gap: var(--space-10); }
  [data-gap='12'] { gap: var(--space-12); }
  [data-gap='16'] { gap: var(--space-16); }
</style>
