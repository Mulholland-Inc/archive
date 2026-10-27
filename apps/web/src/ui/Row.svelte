<script>
  let { label, mark = false, wide = false, items = [], children } = $props();
</script>

<div class="row" class:wide>
  <span class="label">{label}{#if mark}<span class="mark" title="Required" aria-label="Required"></span>{/if}</span>
  <span class="value">{@render children?.()}</span>
  {#if items.length}
    <span class="items">{#each items as item (item)}<span class="item">{item}</span>{/each}</span>
  {/if}
</div>

<style>
  .row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); min-height: var(--size-avatar); font-size: var(--text-sm); }
  .wide { grid-template-columns: minmax(0, 1fr); }
  .label { display: flex; align-items: center; gap: var(--space-2); min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .wide .label { color: var(--text-3); font-size: var(--text-xs); }
  .mark { flex: none; width: var(--size-dot); height: var(--size-dot); border-radius: var(--radius-pill); background: var(--text-3); }
  .value { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: var(--space-1); min-width: 0; color: var(--text-3); font: var(--text-xs)/1 var(--mono); }
  .wide .value { justify-content: flex-start; }
  .items { display: flex; grid-column: 1 / -1; flex-wrap: wrap; gap: var(--space-1); }
  .item { padding: var(--space-2); border-radius: var(--radius-pill); background: var(--chip, var(--raised)); color: var(--text-2); font: var(--text-xs)/1 var(--mono); }
</style>
