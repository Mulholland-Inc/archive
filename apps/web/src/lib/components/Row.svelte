<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    gap = '2',
    align = 'center',
    justify = 'start',
    wrap = false,
    grow = false,
    children
  }: {
    gap?: string;
    align?: 'center' | 'start' | 'end' | 'baseline' | 'stretch';
    justify?: 'start' | 'between' | 'end' | 'center';
    wrap?: boolean;
    grow?: boolean;
    children: Snippet;
  } = $props();
</script>

<div class="row" data-gap={gap} data-align={align} data-justify={justify} data-wrap={wrap || undefined} data-grow={grow || undefined}>
  {@render children()}
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    min-width: 0;
  }
  [data-grow] { flex: 1; }
  [data-wrap] { flex-wrap: wrap; }
  [data-align='start'] { align-items: flex-start; }
  [data-align='end'] { align-items: flex-end; }
  [data-align='baseline'] { align-items: baseline; }
  [data-align='stretch'] { align-items: stretch; }
  [data-justify='between'] { justify-content: space-between; }
  [data-justify='end'] { justify-content: flex-end; }
  [data-justify='center'] { justify-content: center; }
  [data-gap='0'] { gap: 0; }
  [data-gap='0-5'] { gap: var(--space-0-5); }
  [data-gap='1'] { gap: var(--space-1); }
  [data-gap='1-5'] { gap: var(--space-1-5); }
  [data-gap='2'] { gap: var(--space-2); }
  [data-gap='3'] { gap: var(--space-3); }
  [data-gap='4'] { gap: var(--space-4); }
  [data-gap='6'] { gap: var(--space-6); }
  [data-gap='8'] { gap: var(--space-8); }
  [data-gap='12'] { gap: var(--space-12); }
</style>
