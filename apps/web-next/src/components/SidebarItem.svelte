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

  .hasActions:hover .hit,
  .hasActions:focus-within .hit {
    mask-image: linear-gradient(to left, transparent var(--control-lg), black calc(var(--control-lg) + var(--space-6)));
  }

  .active.hasActions .hit {
    mask-image: none;
  }

  .actions {
    position: absolute;
    inset-inline-end: var(--space-0-5);
    display: inline-flex;
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
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
