<script>
  import { titleOf } from '../lib/app.svelte.js';

  // A list: one narrow column of rows, like notes, each its name and the start of its longest text.
  let { view, rows, shown, text, pick } = $props();

  // The body is the long text column, or the text column whose values run longest.
  const body = $derived.by(() => {
    const long = shown.find((c) => c.type === 'long');
    if (long) return long;
    const len = (c) => rows.reduce((n, r) => n + text(c, r).length, 0) / (rows.length || 1);
    const texts = shown.filter((c) => c.type === 'text').sort((a, b) => len(b) - len(a));
    return texts[0] && len(texts[0]) > 24 ? texts[0] : undefined;
  });
</script>

<ul class="list">
  {#each rows as r, i (r.id ?? i)}
    <li>
      <button type="button" class="note" onclick={() => pick(r)}>
        <span class="name">{titleOf(r, view)}</span>
        {#if body && text(body, r)}<span class="body">{text(body, r)}</span>{/if}
      </button>
    </li>
  {/each}
</ul>

<style>
  .list {
    display: flex;
    flex-direction: column;
    width: 100%;
    max-width: var(--view-list-width);
    margin-inline: auto;
  }

  li + li {
    border-top: var(--border-width) solid var(--color-border-subtle);
  }

  /* A note's text starts where the dividers do; its hover reaches out past them. */
  .note {
    display: flex;
    width: calc(100% + 2 * var(--space-3));
    margin-inline: calc(-1 * var(--space-3));
    flex-direction: column;
    gap: var(--space-0-5);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    text-align: start;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .note:hover {
    background: var(--color-fill-subtle);
  }

  .name {
    font-family: var(--font-serif);
    font-size: var(--text-xl);
    line-height: var(--leading-tight);
    color: var(--color-text);
  }

  .body {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
  }
</style>
