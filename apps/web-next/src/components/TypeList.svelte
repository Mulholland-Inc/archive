<script>
  import TypeRow from './TypeRow.svelte';

  let { types = [], all = types, onopen, label = 'Tables' } = $props();
  const names = $derived(Object.fromEntries(all.map((t) => [t.id, t.name])));
</script>

<div class="list" role="list" aria-label={label}>
  <div class="head" aria-hidden="true">
    <span></span>
    <span>Name</span>
    <span class="end">Rows</span>
    <span class="end wide">Columns</span>
    <span class="wide">Links to</span>
    <span class="wide">Updated</span>
    <span></span>
  </div>
  {#each types as type (type.id)}
    <TypeRow {type} links={[...new Set(type.links.map((l) => names[l.to] ?? l.to))]} onclick={() => onopen?.(type.id)} />
  {/each}
</div>

<style>
  .list {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .head {
    display: grid;
    grid-template-columns: var(--type-columns);
    align-items: center;
    gap: var(--space-4);
    height: var(--control-lg);
    padding-inline: var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .end {
    text-align: end;
  }

  @media (max-width: 40rem) {
    .head {
      grid-template-columns: var(--type-columns-narrow);
    }

    .wide {
      display: none;
    }
  }
</style>
