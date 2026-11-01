<script>
  import Icon from './Icon.svelte';

  let { items = [], active, label } = $props();
</script>

<nav class="vnav" aria-label={label}>
  {#each items as item (item.id)}
    {#if item.href}
      <a href={item.href} class={['item', { on: active === item.id }]} aria-current={active === item.id ? 'page' : undefined}>
        {#if item.icon}<Icon icon={item.icon} size="md" />{/if}
        <span>{item.label}</span>
      </a>
    {:else}
      <button type="button" class={['item', { on: active === item.id }]} onclick={item.onclick}>
        {#if item.icon}<Icon icon={item.icon} size="md" />{/if}
        <span>{item.label}</span>
      </button>
    {/if}
  {/each}
</nav>

<style>
  .vnav {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
  }

  .item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: var(--control-lg);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    text-align: start;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .item span {
    padding-inline: var(--space-1);
  }

  .item:hover {
    background: var(--color-fill-subtle);
    color: var(--color-text);
  }

  .on,
  .on:hover {
    background: var(--color-fill);
    color: var(--color-text);
  }

  @media (max-width: 64rem) {
    .vnav {
      flex-direction: row;
      overflow-x: auto;
      overscroll-behavior-y: auto;
    }

    .item {
      flex: none;
    }
  }
</style>
