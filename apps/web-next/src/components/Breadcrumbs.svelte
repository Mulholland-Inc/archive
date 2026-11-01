<script>
  import { ChevronRight } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { items = [], inset = false } = $props();
</script>

<nav aria-label="Breadcrumb">
  <ol class={['crumbs', { inset }]}>
    {#each items as item, i (i)}
      <li>
        {#if item.href && i < items.length - 1}
          <a href={item.href}>{item.label}</a>
        {:else if item.onclick && i < items.length - 1}
          <button type="button" class="link" onclick={item.onclick}>{item.label}</button>
        {:else}
          <span aria-current={i === items.length - 1 ? 'page' : undefined}>{item.label}</span>
        {/if}
      </li>
      {#if i < items.length - 1}
        <li class="sep" aria-hidden="true"><Icon icon={ChevronRight} size="xs" /></li>
      {/if}
    {/each}
  </ol>
</nav>

<style>
  .crumbs {
    display: flex;
    align-items: center;
    gap: var(--space-1-5);
    min-width: 0;
    color: var(--color-text-tertiary);
    font-size: var(--text-md);
  }

  li {
    display: inline-flex;
    min-width: 0;
    white-space: nowrap;
  }

  .inset {
    padding-inline: var(--space-3);
  }

  a,
  .link {
    border-radius: var(--radius-xs);
    transition: color var(--duration-fast) var(--ease-out);
  }

  a:hover,
  .link:hover {
    color: var(--color-text);
  }

  [aria-current='page'] {
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .sep {
    color: var(--color-text-disabled);
  }
</style>
