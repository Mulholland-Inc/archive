<script>
  import { getContext, setContext } from 'svelte';
  import { ChevronRight } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import { uid } from '../lib/ids.js';
  import { show, hide, roving, focusables } from '../lib/floating.js';
  import { duration } from '../lib/motion.js';

  let { icon, label, children } = $props();

  const parent = getContext('menu');
  const id = uid('sub');
  let button = $state();
  let panel = $state();
  let open = $state(false);
  let timer;

  function openSub(focus = false) {
    clearTimeout(timer);
    show(panel, button);
    if (focus) requestAnimationFrame(() => focusables(panel)[0]?.focus());
  }

  function closeSub() {
    hide(panel);
  }

  function hover() {
    clearTimeout(timer);
    timer = setTimeout(() => openSub(false), duration('fast'));
  }

  function leave() {
    clearTimeout(timer);
  }

  function buttonKey(event) {
    if (event.key === 'ArrowRight' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      event.stopPropagation();
      openSub(true);
    }
  }

  function panelKey(event) {
    event.stopPropagation();
    if (event.key === 'ArrowLeft' || event.key === 'Escape') {
      event.preventDefault();
      closeSub();
      button.focus();
      return;
    }
    roving(event, panel, { onclose: parent?.close });
  }

  setContext('menu', { close: () => parent?.close() });
</script>

<button
  bind:this={button}
  type="button"
  role="menuitem"
  tabindex="-1"
  aria-haspopup="menu"
  aria-expanded={open}
  aria-controls={id}
  class={['item', { open }]}
  onclick={() => openSub(true)}
  onkeydown={buttonKey}
  onpointerenter={hover}
  onpointerleave={leave}
>
  {#if icon}<Icon {icon} size="md" />{/if}
  <span class="text">{label}</span>
  <Icon icon={ChevronRight} size="sm" />
</button>

<div
  bind:this={panel}
  {id}
  popover="auto"
  role="menu"
  tabindex="-1"
  aria-label={label}
  class="menu floating"
  data-placement="right-start"
  ontoggle={(e) => (open = e.newState === 'open')}
  onkeydown={panelKey}
>
  {@render children()}
</div>

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
    transition: background-color var(--duration-instant) var(--ease-out);
  }

  .item > :global(.icon) {
    color: var(--color-text-secondary);
  }

  .item:hover,
  .item:focus-visible,
  .open {
    background: var(--color-fill);
    outline: none;
  }

  .text {
    flex: 1;
    padding-inline: var(--space-0-5);
  }

  .menu {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
    min-width: var(--menu-min);
    max-width: var(--menu-max);
    padding: var(--space-1-5);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
    box-shadow: var(--shadow-md);
  }
</style>
