<script>
  import { titleOf } from '../lib/app.svelte.js';
  import { fill } from './fill.js';
  import { label, figure } from './format.js';

  // A gallery: a card for each row, its name over up to four of its other columns, for listings, recipes and
  // products. The cards fill the window and scroll.
  let { view, rows, shown, text, pick } = $props();

  const details = $derived(shown.filter((c) => c.type !== 'file' && c.type !== 'long').slice(0, 4));
</script>

<div class="gallery" {@attach fill()}>
  <ul class="cards">
    {#each rows as r, i (r.id ?? i)}
      <li class="card">
        <button type="button" class="hit" aria-label={titleOf(r, view)} onclick={() => pick(r)}></button>
        <span class="name">{titleOf(r, view)}</span>
        {#if details.some((c) => text(c, r))}
          <dl>
            {#each details.filter((c) => text(c, r)) as c (c.name)}<dt>{label(c)}</dt><dd>{figure(c, r, text)}</dd>{/each}
          </dl>
        {/if}
      </li>
    {/each}
  </ul>
</div>

<style>
  .gallery {
    height: var(--view-fill);
    overflow-y: auto;
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(var(--view-card-width), 1fr));
    gap: var(--space-3);
  }

  .card {
    position: relative;
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .card:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .hit {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    cursor: pointer;
  }

  .name {
    overflow: hidden;
    color: var(--color-text);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  /* Each detail is its name on the left and its value on the right, like a sheet of specs. */
  dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: var(--space-1) var(--space-3);
    font-size: var(--text-sm);
  }

  dt {
    color: var(--color-text-tertiary);
  }

  dd {
    overflow: hidden;
    color: var(--color-text-secondary);
    font-variant-numeric: tabular-nums;
    text-align: end;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
</style>
