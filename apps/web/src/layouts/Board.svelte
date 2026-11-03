<script>
  import Text from '../components/Text.svelte';
  import RowLink from '../components/RowLink.svelte';
  import { linkOf, titleOf } from '../lib/app.svelte.js';
  import { fill } from './fill.js';

  // A board: a lane for each value of the view's by column, a card for each row. The board fills the window
  // and scrolls sideways; each lane is as tall as it and scrolls its own cards.
  let { view, rows, shown, text, pick } = $props();

  const byCol = $derived(shown.find((c) => c.name === view.by));
  const lanes = $derived.by(() => {
    const by = view.by;
    const out = new Map();
    for (const r of rows) {
      const k = r[by] == null || r[by] === '' ? 'None' : byCol ? text(byCol, r) : String(r[by]);
      if (!out.has(k)) out.set(k, []);
      out.get(k).push(r);
    }
    return [...out];
  });
</script>

<div class="board" {@attach fill()}>
  {#each lanes as [lane, items] (lane)}
    <section class="lane">
      <header><Text size="sm" weight="medium">{lane}</Text><Text size="sm" tone="tertiary" tabular>{items.length}</Text></header>
      <div class="cards">
        {#each items as r, i (r.id ?? i)}
          <!-- The card opens its row; its chips, above that, open the rows they name. -->
          <div class="card">
            <button type="button" class="hit" aria-label={titleOf(r, view)} onclick={() => pick(r)}></button>
            <Text size="sm" weight="medium">{titleOf(r, view)}</Text>
            {#each shown.filter((c) => c.name !== view.by && r[c.name] != null && r[c.name] !== '') as c (c.name)}
              {#if linkOf(view, c)}<span class="chips"><RowLink table={linkOf(view, c)} value={r[c.name]} /></span>{:else}<Text size="sm" tone="secondary">{text(c, r)}</Text>{/if}
            {/each}
          </div>
        {/each}
      </div>
    </section>
  {/each}
</div>

<style>
  .chips {
    position: relative;
    z-index: var(--z-raised);
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .board {
    display: grid;
    height: var(--view-fill);
    min-width: 0;
    max-width: 100%;
    padding-block-end: var(--space-2);
    grid-auto-flow: column;
    grid-auto-columns: minmax(var(--view-lane-width), 1fr);
    grid-template-rows: minmax(0, 1fr);
    gap: var(--space-3);
    overflow-x: auto;
  }

  .lane {
    display: flex;
    min-height: 0;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-2);
    border-radius: var(--radius-lg);
    background: var(--color-fill);
  }

  .cards {
    display: flex;
    min-height: 0;
    flex: 1;
    flex-direction: column;
    gap: var(--space-2);
    overflow-y: auto;
  }

  .lane header {
    display: flex;
    justify-content: space-between;
    padding: var(--space-1) var(--space-1-5);
  }

  .card {
    position: relative;
    display: flex;
    width: 100%;
    text-align: start;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .card:hover {
    background: var(--color-surface-raised);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .hit {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    cursor: pointer;
  }
</style>
