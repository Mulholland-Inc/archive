<script>
  import { uid } from '../lib/ids.js';
  import { show, hide } from '../lib/floating.js';
  import { duration } from '../lib/motion.js';
  import Kbd from './Kbd.svelte';

  let { text, shortcut, placement = 'top', size = 'sm', instant = false, block = false, children } = $props();

  const id = uid('tip');
  let tip = $state();
  let anchor = $state();
  let timer;

  function open() {
    clearTimeout(timer);
    timer = setTimeout(() => show(tip, anchor), instant ? 0 : duration('tooltip-delay'));
  }

  function close() {
    clearTimeout(timer);
    hide(tip);
  }

  function focus(event) {
    if (event.target.matches?.(':focus-visible')) show(tip, anchor);
  }

  function listen(node) {
    node.addEventListener('pointerenter', open);
    node.addEventListener('pointerleave', close);
    node.addEventListener('pointerdown', close);
    node.addEventListener('focusin', focus);
    node.addEventListener('focusout', close);
    return {
      destroy() {
        clearTimeout(timer);
        node.removeEventListener('pointerenter', open);
        node.removeEventListener('pointerleave', close);
        node.removeEventListener('pointerdown', close);
        node.removeEventListener('focusin', focus);
        node.removeEventListener('focusout', close);
      }
    };
  }
</script>

<span class={['anchor', { block }]} bind:this={anchor} use:listen>
  {@render children()}
</span>
<span bind:this={tip} {id} popover="manual" role="tooltip" class={['tip', 'floating', size]} data-placement={placement}>
  <span>{text}</span>
  {#if shortcut}
    <Kbd keys={shortcut} subtle />
  {/if}
</span>

<style>
  .anchor {
    display: inline-flex;
    min-width: 0;
  }

  .block {
    display: flex;
  }

  .tip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    max-width: var(--tooltip-max);
    padding: var(--space-1-5);
    border-radius: var(--radius-sm);
    background: var(--color-inverse);
    color: var(--color-text-inverse);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    line-height: var(--leading-snug);
    pointer-events: none;
    box-shadow: var(--ring);
  }

  .tip span {
    padding-inline: var(--space-0-5);
  }

  .md {
    padding: var(--space-2);
    border-radius: var(--radius-md);
    font-size: var(--text-md);
  }

  .tip :global(kbd) {
    color: var(--color-text-inverse);
    opacity: 0.6;
  }
</style>
