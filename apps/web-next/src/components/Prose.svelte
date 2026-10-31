<script>
  import Inline from './Inline.svelte';
  import CodeBlock from './CodeBlock.svelte';

  let { blocks = [] } = $props();
</script>

<div class="prose">
  {#each blocks as block, i (i)}
    {#if block.type === 'h'}
      <h3><Inline text={block.text} /></h3>
    {:else if block.type === 'ul'}
      <ul>
        {#each block.items as item, j (j)}<li><Inline text={item} /></li>{/each}
      </ul>
    {:else if block.type === 'ol'}
      <ol>
        {#each block.items as item, j (j)}<li><Inline text={item} /></li>{/each}
      </ol>
    {:else if block.type === 'quote'}
      <blockquote><Inline text={block.text} /></blockquote>
    {:else if block.type === 'code'}
      <CodeBlock code={block.code} lang={block.lang} />
    {:else if block.type === 'table'}
      <div class="table">
        <table>
          <thead><tr>{#each block.head as h, j (j)}<th>{h}</th>{/each}</tr></thead>
          <tbody>
            {#each block.rows as row, r (r)}
              <tr>{#each row as c, j (j)}<td><Inline text={c} /></td>{/each}</tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p><Inline text={block.text} /></p>
    {/if}
  {/each}
</div>

<style>
  .prose {
    display: flex;
    flex-direction: column;
    gap: var(--space-prose);
    min-width: 0;
    font-family: var(--font-prose);
    font-size: var(--text-prose);
    line-height: var(--leading-prose);
    letter-spacing: var(--tracking-normal);
    color: var(--color-text);
  }

  h3 {
    font-family: var(--font-sans);
    font-size: var(--text-prose-heading);
    font-weight: var(--weight-semibold);
    letter-spacing: var(--tracking-snug);
    line-height: var(--leading-snug);
  }

  h3:not(:first-child) {
    padding-block-start: var(--space-2);
  }

  ul,
  ol {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding-inline-start: var(--space-5);
  }

  ul {
    list-style-type: disc;
  }

  ol {
    list-style-type: decimal;
  }

  li::marker {
    color: var(--color-text-tertiary);
  }

  ol li {
    padding-inline-start: var(--space-1);
  }

  blockquote {
    padding: var(--space-1);
    padding-inline-start: var(--space-4);
    box-shadow: inset var(--focus-width) 0 0 var(--color-border-strong);
    color: var(--color-text-secondary);
    font: italic var(--weight-serif) var(--text-prose-quote) / var(--leading-normal) var(--font-serif);
  }

  .table {
    overflow-x: auto;
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  table {
    width: 100%;
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    line-height: var(--leading-snug);
    letter-spacing: var(--tracking-snug);
  }

  th,
  td {
    padding: var(--space-3);
    text-align: start;
    vertical-align: top;
  }

  th {
    color: var(--color-text-secondary);
    font-weight: var(--weight-medium);
    background: var(--color-fill-subtle);
  }

  tr + tr td,
  tbody tr:first-child td {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }
</style>
