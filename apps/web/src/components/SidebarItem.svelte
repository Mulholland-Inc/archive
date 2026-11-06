<script>
  import { getContext } from 'svelte';
  import Icon from './Icon.svelte';
  import Tooltip from './Tooltip.svelte';
  import InlineEdit from './InlineEdit.svelte';
  import { contextmenu } from '../lib/contextmenu.svelte.js';

  let { href, icon, label, active = false, onclick, accent = false, leading, badge, tall = false, detail, dot = false, ondouble, items, onrename, editing = $bindable(false) } = $props();

  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

{#snippet row()}
  <div class={['item', { active, collapsed, accent, tall }]} {@attach items && !editing && contextmenu(items)}>
    {#if editing && onrename}
      <InlineEdit value={label} label={`Rename ${label}`} onsave={(title) => { editing = false; onrename(title); }} oncancel={() => (editing = false)} />
    {:else if href}
      <a class="hit" {href} aria-current={active ? 'page' : undefined} aria-label={collapsed ? label : undefined} ondblclick={ondouble ? (event) => { event.preventDefault(); ondouble(); } : onrename ? (event) => { event.preventDefault(); editing = true; } : undefined}>
        {@render content()}
      </a>
    {:else}
      <button type="button" class="hit" {onclick} aria-label={collapsed ? label : undefined}>
        {@render content()}
      </button>
    {/if}
  </div>
{/snippet}

{#snippet content()}
  {#if leading}
    <span class="glyph">{@render leading()}</span>
  {:else if icon}
    <span class="glyph"><Icon {icon} size="md" /></span>
  {/if}
  {#if tall}
    <span class="text">
      <span class="label">{label}</span>
      {#if detail}<span class="detail">{detail}</span>{/if}
    </span>
  {:else}
    <span class="label">{label}</span>
  {/if}
  {#if badge && !collapsed}<span class="badge">{badge}</span>{/if}
  {#if dot}<i class="dot" role="img" aria-label="New message"></i>{/if}
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
  .item:focus-within,
  .item:global([data-contextmenu]) {
    background: var(--color-fill);
    color: var(--color-text);
  }

  .active,
  .active:hover {
    background: var(--color-fill-strong);
    color: var(--color-text);
  }

  .hit {
    touch-action: manipulation;
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

  .tall .hit {
    height: auto;
    padding: var(--space-1) var(--space-1-5);
  }

  .tall .glyph {
    width: auto;
    height: auto;
  }

  .text {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
    line-height: var(--leading-snug);
  }

  .tall .label {
    flex: none;
    color: var(--color-text);
  }

  .detail {
    overflow: hidden;
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    text-overflow: ellipsis;
  }

  .dot {
    flex: none;
    width: var(--space-2);
    height: var(--space-2);
    border-radius: var(--radius-full);
    background: var(--led-blue);
  }

  .collapsed .dot {
    position: absolute;
    inset-block-start: var(--space-0-5);
    inset-inline-end: var(--space-0-5);
    box-shadow: 0 0 0 var(--space-0-5) var(--color-sidebar);
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

  .badge {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

</style>
