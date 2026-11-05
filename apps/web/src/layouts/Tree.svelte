<script>
  import { titleOf, linkOf, baseOf } from '../lib/app.svelte.js';
  import { fill } from './fill.js';
  import { label, figure } from './format.js';

  let { view, rows, shown, text, pick } = $props();

  const byCol = $derived(view.columns.find((c) => c.name === view.by));
  const self = $derived(Boolean(byCol) && [view.id, baseOf(view)].includes(linkOf(view, byCol)));
  const details = $derived(shown.filter((c) => c.name !== view.by && c.type !== 'long' && c.type !== 'file').slice(0, 3));

  const lines = $derived.by(() => {
    const ids = new Set(rows.map((r) => r.id));
    const parent = (r) => (self && r[view.by] !== r.id && ids.has(r[view.by]) ? r[view.by] : null);
    const under = new Map();
    for (const r of rows) {
      const p = parent(r);
      if (!under.has(p)) under.set(p, []);
      under.get(p).push(r);
    }
    const out = [];
    const seen = new Set();
    const walk = (list, guides) =>
      list.forEach((r, i) => {
        if (seen.has(r.id)) return;
        seen.add(r.id);
        const last = i === list.length - 1;
        out.push({ r, guides, last });
        walk(under.get(r.id) ?? [], [...guides, !last]);
      });
    walk(under.get(null) ?? [], []);
    walk(rows.filter((r) => !seen.has(r.id)), []);
    return out;
  });
</script>

<div class="tree" {@attach fill()} style:--details={details.length}>
  <div class="row head">
    <span>Name</span>
    {#each details as c (c.name)}<span class={{ end: c.type === 'money' || c.type === 'number' }}>{label(c)}</span>{/each}
  </div>
  <div class="scroll">
    {#each lines as { r, guides, last }, i (r.id ?? i)}
      <button type="button" class="row item" onclick={() => pick(r)}>
        <span class="node">
          {#each guides.slice(1) as on, d (d)}<span class={['guide', { on }]}></span>{/each}
          {#if guides.length}<span class={['guide', 'elbow', { last }]}></span>{/if}
          <span class={['name', { root: !guides.length }]}>{titleOf(r, view)}</span>
        </span>
        {#each details as c (c.name)}<span class={['detail', { end: c.type === 'money' || c.type === 'number' }]}>{figure(c, r, text)}</span>{/each}
      </button>
    {/each}
  </div>
</div>

<style>
  .tree {
    display: flex;
    height: var(--view-fill);
    flex-direction: column;
    overflow-x: auto;
  }

  .row {
    display: grid;
    min-width: min-content;
    grid-template-columns: minmax(var(--view-tree-name), 1fr) repeat(var(--details), var(--view-tree-detail));
    gap: var(--space-8);
    align-items: center;
  }

  .head {
    height: var(--control-lg);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .end {
    text-align: end;
  }

  .scroll {
    min-height: 0;
    flex: 1;
    margin-inline: calc(-1 * var(--space-3));
    padding-inline: var(--space-3);
    overflow-y: auto;
  }

  .item {
    width: calc(100% + 2 * var(--space-3));
    height: var(--view-tree-row);
    margin-inline: calc(-1 * var(--space-3));
    padding-inline: var(--space-3);
    border-radius: var(--radius-md);
    font-size: var(--text-md);
    text-align: start;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .item:hover {
    background: var(--color-fill-subtle);
  }

  .node {
    display: flex;
    min-width: 0;
    align-self: stretch;
    align-items: center;
  }

  .guide {
    position: relative;
    flex: none;
    align-self: stretch;
    width: var(--view-tree-indent);
  }

  .guide.on::before,
  .elbow::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: var(--view-tree-line);
    width: var(--border-width);
    background: var(--color-text-disabled);
  }

  .elbow.last::before {
    display: none;
  }

  .elbow::after {
    content: '';
    position: absolute;
    top: calc(50% - var(--border-width));
    right: var(--space-2);
    left: calc(var(--view-tree-line) + var(--border-width));
    height: var(--border-width);
    background: var(--color-text-disabled);
  }

  .elbow.last::after {
    top: 0;
    left: var(--view-tree-line);
    height: 50%;
    border-bottom-left-radius: var(--radius-xs);
    background: none;
    box-shadow: inset var(--border-width) calc(var(--border-width) * -1) 0 var(--color-text-disabled);
  }

  .name {
    overflow: hidden;
    color: var(--color-text);
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .root {
    font-weight: var(--weight-medium);
  }

  .detail {
    overflow: hidden;
    color: var(--color-text-secondary);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
</style>
