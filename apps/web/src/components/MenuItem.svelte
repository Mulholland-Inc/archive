<script>
  import { getContext } from 'svelte';
  import { Check } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import Kbd from './Kbd.svelte';
  import Switch from './Switch.svelte';
  import { go } from '../lib/router.svelte.js';

  let { icon, shortcut, danger = false, checked, toggle, disabled = false, href, onselect, keepOpen = false, leading, end, children } = $props();

  const menu = getContext('menu');

  function activate(event) {
    if (disabled) return;
    onselect?.(event);
    if (!keepOpen) menu?.close();
    if (href) go(href);
  }
</script>

<button
  type="button"
  role={checked === undefined && toggle === undefined ? 'menuitem' : 'menuitemcheckbox'}
  aria-checked={checked ?? toggle}
  data-toggle={toggle !== undefined || undefined}
  aria-disabled={disabled || undefined}
  tabindex="-1"
  class={['item', { danger, disabled }]}
  onclick={activate}
>
  {#if leading}
    {@render leading()}
  {:else if icon}
    <Icon {icon} size="md" />
  {/if}
  <span class="text">{@render children()}</span>
  {#if end}{@render end()}{/if}
  {#if shortcut}<Kbd keys={shortcut} subtle />{/if}
  {#if checked}<span class="check"><Icon icon={Check} size="md" /></span>{/if}
  {#if toggle !== undefined}<Switch visual checked={toggle} size="sm" />{/if}
</button>

<style>
  .item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-height: var(--control-md);
    padding: var(--space-1-5);
    border-radius: var(--radius-md);
    color: var(--color-text);
    text-align: start;
    line-height: var(--leading-snug);
    transition: background-color var(--duration-instant) var(--ease-out);
  }

  .item > :global(.icon) {
    color: var(--color-text-secondary);
  }

  .item:hover,
  .item:focus-visible {
    background: var(--color-fill);
    outline: none;
  }

  .text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding-inline: var(--space-0-5);
  }

  .check {
    display: inline-flex;
    color: var(--color-accent);
  }

  .danger,
  .danger > :global(.icon) {
    color: var(--color-danger);
  }

  .danger:hover,
  .danger:focus-visible {
    background: var(--color-fill);
  }

  .disabled {
    opacity: var(--opacity-disabled);
    cursor: default;
  }

  .disabled:hover {
    background: none;
  }
</style>
