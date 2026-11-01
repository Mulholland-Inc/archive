<script>
  import { X } from '@lucide/svelte';
  import Art from './Art.svelte';
  import IconButton from './IconButton.svelte';
  import { app } from '../lib/app.svelte.js';

  let { art, label = 'Details', side, children } = $props();

  let aside = $state();
  let previous = null;

  $effect(() => {
    if (app.details) {
      previous = document.activeElement;
      requestAnimationFrame(() => aside?.querySelector('.close button')?.focus());
    } else if (previous) {
      if (previous.isConnected) previous.focus();
      previous = null;
    }
  });

  $effect(() => () => (app.details = false));

  function onkeydown(event) {
    if (app.details && event.key === 'Escape' && !event.defaultPrevented && !document.querySelector('dialog[open]')) app.details = false;
  }
</script>

<svelte:window {onkeydown} />

<div class="layout">
  <div class="content" inert={app.details}>{@render children()}</div>
  <aside class={['side', { open: app.details }]} aria-label={label} bind:this={aside}>
    {#if art}<div class="art"><Art name={art} size="zoom" /></div>{/if}
    <span class="close"><IconButton icon={X} label="Close details" tooltip={false} onclick={() => (app.details = false)} /></span>
    <div class="body">{@render side()}</div>
  </aside>
</div>

<style>
  .layout {
    display: flex;
    flex: 1;
    min-width: 0;
    min-height: 0;
  }

  .content {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    min-height: 0;
  }

  .side {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: none;
    width: var(--aside-width);
    overflow-y: auto;
    overflow-x: hidden;
    overscroll-behavior: contain;
    background: var(--color-sidebar);
    box-shadow: inset var(--border-width) 0 0 var(--color-border-subtle);
    transition: background-color var(--duration-normal) var(--ease-out);
  }

  .art {
    display: grid;
    place-items: center;
    flex: none;
    height: var(--aside-art);
    margin-block-end: calc(var(--space-8) * -1);
    overflow: hidden;
    mask-image: linear-gradient(to bottom, #000 35%, transparent);
    pointer-events: none;
  }

  .art :global(.art) {
    mix-blend-mode: multiply;
  }

  :global([data-theme='dark']) .art :global(.art) {
    mix-blend-mode: lighten;
  }

  .close {
    display: none;
    position: absolute;
    inset-block-start: var(--space-2);
    inset-inline-end: var(--space-2);
    z-index: var(--z-raised);
  }

  .body {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-3) var(--space-3) var(--space-6);
  }

  @media (max-width: 56rem) {
    .side {
      position: fixed;
      inset: 0;
      z-index: var(--z-overlay);
      width: auto;
      box-shadow: none;
      transform: translateX(100%);
      visibility: hidden;
      transition:
        transform var(--duration-slow) var(--ease-in-out),
        visibility var(--duration-slow) var(--ease-in-out);
    }

    .open {
      transform: none;
      visibility: visible;
      transition:
        transform var(--duration-slow) var(--ease-in-out),
        visibility 0s;
    }

    .close {
      display: flex;
    }

    .body {
      padding-inline: var(--space-4);
    }
  }
</style>
