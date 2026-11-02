<script>
  import { typeLabel } from '../lib/schema.js';

  let { type, names = {}, mode = 'idle', onclick, onhover, onmeasure } = $props();

  let node = $state();
  let head = $state();
  let rows = $state([]);
  let width = $state(0);
  let height = $state(0);
  const columns = $derived(type.columns.filter((c) => c.name !== 'id' && c.name !== 'created_at'));

  $effect(() => {
    if (!width || !height || !head) return;
    const anchors = { '': head.offsetTop + head.offsetHeight / 2 };
    columns.forEach((c, i) => {
      const el = rows[i];
      if (el) anchors[c.name] = el.offsetTop + el.offsetHeight / 2;
    });
    onmeasure?.({ w: width, h: height, anchors });
  });
</script>

<button
  type="button"
  class={['node', mode]}
  bind:this={node}
  bind:offsetWidth={width}
  bind:offsetHeight={height}
  onclick={onclick}
  onpointerenter={() => onhover?.(type.id)}
  onpointerleave={() => onhover?.(null)}
  onfocus={() => onhover?.(type.id)}
  onblur={() => onhover?.(null)}
>
  <span class="head" bind:this={head}>
    <span class="name">{type.name}</span>
    <span class="count">{type.rows?.toLocaleString('en-US') ?? '—'} rows</span>
  </span>
  <span class="columns">
    {#each columns as c, i (c.name)}
      <span class="column" bind:this={rows[i]}>
        <span class="label">{c.name}</span>
        {#if c.to}
          <span class="target">{names[c.to] ?? c.to}</span>
        {:else}
          <span class="kind">{typeLabel(c)}</span>
        {/if}
      </span>
    {/each}
  </span>
</button>

<style>
  .node {
    display: flex;
    flex-direction: column;
    width: var(--schema-node);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    text-align: start;
    transition:
      box-shadow var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out);
  }

  .node:hover,
  .active,
  .match {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .match {
    box-shadow: inset 0 0 0 var(--border-width-thick) var(--color-accent);
  }

  .head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-2-5) var(--space-3);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .count {
    flex: none;
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .columns {
    display: flex;
    flex-direction: column;
    padding-block: var(--space-1);
  }

  .column {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    min-height: var(--control-sm);
    padding-inline: var(--space-3);
    font-size: var(--text-sm);
  }

  .label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-text-secondary);
  }

  .kind {
    flex: none;
    color: var(--color-text-tertiary);
  }

  .target {
    flex: none;
    color: var(--color-text);
    font-weight: var(--weight-medium);
  }
</style>
