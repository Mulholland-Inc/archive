<script>
  import { getContext } from 'svelte';
  import Icon from './Icon.svelte';
  import Tooltip from './Tooltip.svelte';
  import Kbd from './Kbd.svelte';

  let { href, icon, label, active = false, onclick, shortcut, accent = false, leading, actions, badge } = $props();

  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

{#snippet row()}
  <div class={['item', { active, collapsed, accent, hasActions: actions }]}>
    {#if href}
      <a class="hit" {href} aria-current={active ? 'page' : undefined} aria-label={collapsed ? label : undefined}>
        {@render content()}
      </a>
    {:else}
      <button type="button" class="hit" {onclick} aria-label={collapsed ? label : undefined}>
        {@render content()}
      </button>
    {/if}
    {#if actions && !collapsed}<span class="actions">{@render actions()}</span>{/if}
  </div>
{/snippet}

{#snippet content()}
  {#if leading}
    {@render leading()}
  {:else if icon}
    <span class="glyph"><Icon {icon} size="md" /></span>
  {/if}
  <span class="label">{label}</span>
  {#if badge && !collapsed}<span class="badge">{badge}</span>{/if}
  {#if shortcut && !collapsed}<span class="kbd"><Kbd keys={shortcut} subtle /></span>{/if}
{/snippet}

{#if collapsed}
  <Tooltip text={label} placement="right" block>{@render row()}</Tooltip>
{:else}
  {@render row()}
{/if}

<style>
  .item {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
    min-height: var(--control-md);
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out);
  }

  .item:hover,
  .item:focus-within {
    background: var(--color-fill);
    color: var(--color-text);
  }

  .active,
  .active:hover {
    background: var(--color-fill-strong);
    color: var(--color-text);
  }

  .hit {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
    min-width: 0;
    height: var(--control-md);
    padding: var(--space-1-5);
    border-radius: inherit;
    text-align: start;
    font-size: var(--text-md);
    white-space: nowrap;
  }

  .glyph {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--icon-lg);
    height: var(--icon-lg);
  }

  .accent .glyph {
    border-radius: var(--radius-full);
    background: var(--color-fill-strong);
    color: var(--color-text);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .accent .glyph :global(.icon) {
    width: var(--icon-sm);
    height: var(--icon-sm);
  }

  .accent {
    color: var(--color-text);
  }

  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .collapsed .label {
    opacity: 0;
  }

  .item {
    --row-fill: transparent;
  }

  .item:hover,
  .item:focus-within {
    --row-fill: var(--color-fill);
  }

  .active,
  .active:hover {
    --row-fill: var(--color-fill-strong);
  }

  .actions {
    position: absolute;
    inset-block: 0;
    inset-inline-end: 0;
    display: inline-flex;
    align-items: center;
    padding-inline: var(--space-0-5);
    border-start-end-radius: inherit;
    border-end-end-radius: inherit;
    background: linear-gradient(var(--row-fill), var(--row-fill)), var(--color-sidebar);
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .actions::before {
    content: '';
    position: absolute;
    inset-block: 0;
    inset-inline-end: 100%;
    width: var(--space-6);
    background: inherit;
    mask-image: linear-gradient(to right, transparent, black);
  }

  .item:hover .actions,
  .item:focus-within .actions,
  .actions:has(:global([aria-expanded='true'])) {
    opacity: 1;
  }

  .badge {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  .kbd {
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .item:hover .kbd {
    opacity: 1;
  }
</style>
