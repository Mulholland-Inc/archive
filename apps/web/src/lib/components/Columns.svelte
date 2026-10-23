<script lang="ts">
  import type { Snippet } from 'svelte';
  let { count = 2, trailing = false, children }: { count?: 2 | 3 | 4; trailing?: boolean; children: Snippet } = $props();
</script>

<div class="columns" data-count={count} data-trailing={trailing || undefined}>{@render children()}</div>

<style>
  .columns {
    display: grid;
    gap: var(--space-3);
    align-items: end;
    min-width: 0;
  }
  [data-count='2'] { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  [data-count='3'] { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  [data-count='4'] { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  [data-count='2'][data-trailing] { grid-template-columns: repeat(2, minmax(0, 1fr)) auto; }
  [data-count='3'][data-trailing] { grid-template-columns: repeat(3, minmax(0, 1fr)) auto; }
  @media (max-width: 40rem) {
    .columns[data-count] {
      grid-template-columns: minmax(0, 1fr);
    }
    .columns[data-trailing] {
      grid-template-columns: minmax(0, 1fr) auto;
    }
  }
</style>
