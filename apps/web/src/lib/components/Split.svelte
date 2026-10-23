<script lang="ts">
  import type { Snippet } from 'svelte';
  let { main, aside }: { main: Snippet; aside: Snippet } = $props();
</script>

<div class="split">
  <div class="main">{@render main()}</div>
  <aside class="aside">{@render aside()}</aside>
</div>

<style>
  .split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, var(--split-aside));
    gap: var(--space-5);
    align-items: start;
  }
  .main,
  .aside {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    min-width: 0;
  }
  @media (max-width: 64rem) {
    .split {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
