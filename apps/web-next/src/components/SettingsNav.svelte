<script>
  import { ChevronRight, ArrowUpRight } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { items = [], active, query = '', label = 'Settings', onselect } = $props();

  const groups = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = [];
    for (const item of items.filter((i) => !q || i.label.toLowerCase().includes(q) || i.group?.toLowerCase().includes(q))) {
      const last = out.at(-1);
      if (last && last.name === item.group) last.items.push(item);
      else out.push({ name: item.group, items: [item] });
    }
    return out;
  });
</script>

<nav class="nav" aria-label={label}>
  {#each groups as group, i (group.name ?? i)}
    <div class="group">
      {#if group.name}<span class="heading">{group.name}</span>{/if}
      {#each group.items as item (item.id)}
        {#if item.href}
          <a class="item" href={item.href} target={item.external ? '_blank' : undefined} rel={item.external ? 'noreferrer' : undefined} onclick={() => onselect?.(item.id)}>
            <Icon icon={item.icon} size="md" />
            <span class="label">{item.label}</span>
            <span class="arrow"><Icon icon={ArrowUpRight} size="sm" /></span>
          </a>
        {:else}
          <button type="button" class={['item', { on: active === item.id }]} aria-current={active === item.id ? 'page' : undefined} onclick={() => onselect?.(item.id)}>
            <Icon icon={item.icon} size="md" />
            <span class="label">{item.label}</span>
            <span class="chevron"><Icon icon={ChevronRight} size="sm" /></span>
          </button>
        {/if}
      {/each}
    </div>
  {:else}
    <span class="empty">No settings match</span>
  {/each}
</nav>

<style>
  .nav {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
  }

  .heading {
    padding: var(--space-1) var(--space-2) var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }

  .empty {
    padding: var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-md);
  }

  .item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: var(--control-lg);
    padding: var(--space-2) var(--space-2-5);
    border-radius: var(--radius-md);
    color: var(--color-text);
    font-size: var(--text-md);
    text-align: start;
    text-decoration: none;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .item :global(svg) {
    color: var(--color-text-secondary);
  }

  .label {
    flex: 1;
    min-width: 0;
    padding-inline: var(--space-1);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .arrow {
    display: flex;
  }

  .chevron {
    display: none;
  }

  .item:hover {
    background: var(--color-fill-subtle);
  }

  .on,
  .on:hover {
    background: var(--color-fill);
  }

  @media (max-width: 56rem) {
    .item {
      height: var(--list-row);
    }

    .on {
      background: transparent;
    }

    .chevron {
      display: flex;
    }
  }
</style>
