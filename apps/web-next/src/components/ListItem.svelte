<script>
  let { href, onclick, active = false, leading, trailing, meta, title, size = 'md', children } = $props();
  const interactive = $derived(Boolean(href || onclick));
</script>

{#snippet inner()}
  {#if leading}<span class="leading">{@render leading()}</span>{/if}
  <span class="main">
    <span class="title">{#if title}{title}{:else}{@render children?.()}{/if}</span>
    {#if meta}<span class="meta">{@render meta()}</span>{/if}
  </span>
{/snippet}

<li class={['li', size, { active, interactive }]}>
  {#if href}
    <a class="row" {href}>{@render inner()}</a>
  {:else if onclick}
    <button type="button" class="row" {onclick}>{@render inner()}</button>
  {:else}
    <div class="row">{@render inner()}</div>
  {/if}
  {#if trailing}<span class="trailing">{@render trailing()}</span>{/if}
</li>

<style>
  .li {
    position: relative;
    display: flex;
    align-items: center;
    min-height: var(--list-row);
    border-radius: var(--radius-lg);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .sm {
    min-height: var(--control-xl);
  }

  .interactive:hover,
  .active {
    background: var(--color-fill-subtle);
  }

  .interactive:hover + :global(li),
  .active + :global(li) {
    box-shadow: none;
  }

  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex: 1;
    min-width: 0;
    align-self: stretch;
    padding: var(--space-3);
    border-radius: inherit;
    text-align: start;
  }

  .sm .row {
    padding: var(--space-2);
  }

  .leading {
    display: inline-flex;
    flex: none;
    color: var(--color-text-secondary);
  }

  .main {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    min-width: 0;
    flex: 1;
  }

  .title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-text);
    font-weight: var(--weight-medium);
  }

  .meta {
    display: flex;
    align-items: center;
    gap: var(--space-1-5);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    overflow: hidden;
    white-space: nowrap;
  }

  .trailing {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    padding: var(--space-3);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }
</style>
