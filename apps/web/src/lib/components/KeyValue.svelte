<script lang="ts" generics="T">
  import type { Snippet } from 'svelte';
  let { items, label, value }: { items: T[]; label: (item: T) => string; value: Snippet<[T]> } = $props();
</script>

<dl class="list">
  {#each items as item (label(item))}
    <div class="row">
      <dt>{label(item)}</dt>
      <dd>{@render value(item)}</dd>
    </div>
  {/each}
</dl>

<style>
  .list {
    display: flex;
    flex-direction: column;
  }
  .row {
    display: grid;
    grid-template-columns: var(--label-width) minmax(0, 1fr);
    gap: var(--space-4);
    padding: var(--space-2-5) 0;
    border-bottom: var(--border-width) solid var(--border);
  }
  .row:last-child {
    border-bottom: none;
  }
  dt {
    color: var(--muted-foreground);
    overflow-wrap: anywhere;
  }
  dd {
    min-width: 0;
    overflow-wrap: anywhere;
  }
</style>
