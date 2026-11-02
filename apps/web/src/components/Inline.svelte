<script>
  import { inline } from '../lib/inline.js';

  let { text = '' } = $props();
  const parts = $derived(inline(text));
</script>

{#each parts as part, i (i)}
  {#if part.type === 'strong'}<strong>{part.text}</strong>{:else if part.type === 'em'}<em>{part.text}</em>{:else if part.type === 'code'}<code>{part.text}</code>{:else if part.type === 'link'}<a href={part.url}>{part.text}</a>{:else}{part.text}{/if}
{/each}

<style>
  strong {
    font-weight: var(--weight-semibold);
  }

  code {
    padding: var(--space-0-5);
    border-radius: var(--radius-xs);
    background: var(--color-fill);
    font-family: var(--font-sans);
    font-size: var(--text-inline-code);
    letter-spacing: var(--tracking-normal);
  }

  a {
    color: var(--color-accent);
    text-decoration: underline;
    text-decoration-color: var(--color-accent-subtle);
    text-underline-offset: var(--space-0-5);
  }
</style>
