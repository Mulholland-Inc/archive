<script>
  import { ChevronRight } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { items = [], active, label = 'Settings', onselect } = $props();
</script>

<nav class="nav" aria-label={label}>
  {#each items as item (item.id)}
    <button type="button" class={['item', { on: active === item.id }]} aria-current={active === item.id ? 'page' : undefined} onclick={() => onselect?.(item.id)}>
      <Icon icon={item.icon} size="md" />
      <span class="label">{item.label}</span>
      <span class="chevron"><Icon icon={ChevronRight} size="sm" /></span>
    </button>
  {/each}
</nav>

<style>
  .nav {
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

  .label {
    flex: 1;
    padding-inline: var(--space-1);
  }

  .chevron {
    display: none;
    color: var(--color-text-tertiary);
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

  @media (max-width: 56rem) {
    .item {
      height: var(--list-row);
      padding-inline: var(--space-3);
    }

    .on {
      background: transparent;
      color: var(--color-text-secondary);
    }

    .chevron {
      display: flex;
    }
  }
</style>
