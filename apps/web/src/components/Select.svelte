<script>
  import { Check, ChevronDown } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import { uid } from '../lib/ids.js';
  import { show, hide, roving, focusables } from '../lib/floating.js';

  let {
    value = $bindable(),
    options = [],
    placeholder = 'Select',
    size = 'md',
    variant = 'field',
    label,
    id = uid('select'),
    disabled = false,
    full = false,
    placement = 'bottom-start',
    onchange
  } = $props();

  const listId = uid('list');
  let button = $state();
  let panel = $state();
  let open = $state(false);
  const current = $derived(options.find((o) => o.value === value));

  function openList() {
    show(panel, button);
    requestAnimationFrame(() => {
      const items = focusables(panel);
      (items.find((el) => el.getAttribute('aria-selected') === 'true') ?? items[0])?.focus();
    });
  }

  function pick(v) {
    value = v;
    onchange?.(v);
    hide(panel);
    button.focus();
  }

  function buttonKey(event) {
    if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(event.key)) {
      event.preventDefault();
      openList();
    }
  }
</script>

<button
  bind:this={button}
  {id}
  type="button"
  class={['trigger', size, variant, { full, open }]}
  aria-haspopup="listbox"
  aria-expanded={open}
  aria-controls={listId}
  aria-label={label}
  {disabled}
  onclick={() => (open ? hide(panel) : openList())}
  onkeydown={buttonKey}
>
  {#if current?.icon}<Icon icon={current.icon} size="sm" />{/if}
  <span class={['value', { placeholder: !current }]}>{current?.label ?? placeholder}</span>
  <Icon icon={ChevronDown} size="sm" />
</button>

<div
  bind:this={panel}
  id={listId}
  popover="auto"
  role="listbox"
  tabindex="-1"
  aria-label={label}
  class="list floating"
  data-placement={placement}
  data-match
  ontoggle={(e) => (open = e.newState === 'open')}
  onkeydown={(e) => roving(e, panel, { onclose: () => hide(panel) })}
>
  {#each options as option (option.value)}
    <button
      type="button"
      role="option"
      aria-selected={option.value === value}
      tabindex="-1"
      class="option"
      onclick={() => pick(option.value)}
    >
      {#if option.icon}<Icon icon={option.icon} size="md" />{/if}
      <span class="text">{option.label}</span>
      {#if option.value === value}<span class="check"><Icon icon={Check} size="md" /></span>{/if}
    </button>
  {/each}
</div>

<style>
  .trigger {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    width: fit-content;
    height: var(--control-xl);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    color: var(--color-text-tertiary);
    text-align: start;
    transition: box-shadow var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
  }

  .trigger:hover:not(:disabled) {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .trigger:disabled {
    opacity: var(--opacity-disabled-field);
  }

  .open {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .sm {
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
  }

  .ghost {
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    background: none;
    box-shadow: none;
    color: var(--color-text-secondary);
  }

  .ghost:hover:not(:disabled),
  .ghost.open {
    background: var(--color-fill);
    box-shadow: none;
    color: var(--color-text);
  }

  .full {
    display: flex;
    width: 100%;
  }

  .value {
    flex: 1;
    min-width: 0;
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ghost .value {
    color: inherit;
    font-weight: var(--weight-medium);
  }

  .placeholder {
    color: var(--color-text-tertiary);
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
    min-width: var(--menu-min);
    max-height: var(--listbox-max-height);
    overflow-y: auto;
    padding: var(--space-1-5);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
  }

  .option {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--control-md);
    padding: var(--space-1-5);
    border-radius: var(--radius-md);
    text-align: start;
  }

  .option > :global(.icon) {
    color: var(--color-text-secondary);
  }

  .option:hover,
  .option:focus-visible {
    background: var(--color-fill);
    outline: none;
  }

  .text {
    flex: 1;
    padding-inline: var(--space-0-5);
    white-space: nowrap;
  }

  .check {
    display: inline-flex;
    color: var(--color-accent);
  }
</style>
