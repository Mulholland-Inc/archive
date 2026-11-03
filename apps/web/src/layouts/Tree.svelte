<script>
  import { titleOf, linkOf, baseOf } from '../lib/app.svelte.js';
  import { fill } from './fill.js';
  import { label, figure } from './format.js';

  // A tree: each row under the row its by column links to, an outline with lines from each row to those under
  // it, for an org chart, a family, folders or categories. A row whose by is empty, or names a row not here, is
  // a root. Beside each name are up to three of its other columns, in columns of their own. The names stay at
  // the top and the rows scroll under them.
  let { view, rows, shown, text, pick } = $props();

  const byCol = $derived(view.columns.find((c) => c.name === view.by));
  // The by column links rows of the view to rows of it, or of the table it reads.
  const self = $derived(Boolean(byCol) && [view.id, baseOf(view)].includes(linkOf(view, byCol)));
  const details = $derived(shown.filter((c) => c.name !== view.by && c.type !== 'long' && c.type !== 'file').slice(0, 3));

  // The rows in outline order, each with its depth, whether it is the last under its parent, and for each row
  // above it whether more come under that one after it, which is where a line runs on down.
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
    // Rows that only lead back to each other have no root; they are roots of their own.
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
  /* On a page too narrow for the names and their details, the tree scrolls sideways rather than squeeze the
     names. */
  .tree {
    display: flex;
    height: var(--view-fill);
    flex-direction: column;
    overflow-x: auto;
  }

  /* Every row, the head's too, has the same columns: the outline, then each detail. */
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

  /* The rows scroll in a box as much wider than the tree as a row's hover reaches out past it. */
  .scroll {
    min-height: 0;
    flex: 1;
    margin-inline: calc(-1 * var(--space-3));
    padding-inline: var(--space-3);
    overflow-y: auto;
  }

  /* A row's text starts where the head's does; its hover reaches out past it. */
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

  /* A guide is one step of indent. Its line runs down from under the start of the name of the row it belongs
     to: on through the step when more rows come under that one, and in the last step curving into the name it
     leads to, stopping short of it. */
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

  /* Each line is drawn once, as its color is see-through and would darken where two overlap: a row with more
     after it branches straight off the line running on, and the last one is the line's own end, curving in. */
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
