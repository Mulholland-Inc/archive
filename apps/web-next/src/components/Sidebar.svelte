<script>
  import { setContext } from 'svelte';

  let { collapsed = false, drawer = false, rail, header, footer, children, onclose } = $props();

  setContext('sidebar', {
    get collapsed() {
      return collapsed && !drawer;
    }
  });
</script>

{#if drawer}
  <button type="button" class="scrim" aria-label="Close sidebar" tabindex="-1" onclick={onclose}></button>
{/if}
<aside class={['sidebar', { collapsed, drawer, railed: !!rail }]} aria-label="Sidebar">
  {@render rail?.()}
  <div class="column">
    <div class="head">{@render header?.()}</div>
    <nav class="body">{@render children()}</nav>
    <div class="foot">{@render footer?.()}</div>
  </div>
</aside>

<style>
  .sidebar {
    position: relative;
    --rail-space: 0rem;
    z-index: var(--z-sidebar);
    display: flex;
    flex: none;
    width: calc(var(--sidebar-width) + var(--rail-space));
    height: 100%;
    background: var(--color-sidebar);
    box-shadow: inset calc(var(--border-width) * -1) 0 0 var(--color-border-subtle);
    transition:
      width var(--duration-slow) var(--ease-in-out),
      transform var(--duration-slow) var(--ease-in-out),
      background-color var(--duration-normal) var(--ease-out);
    overflow: hidden;
  }

  .railed {
    --rail-space: var(--vault-rail);
  }

  .collapsed {
    width: calc(var(--sidebar-rail) + var(--rail-space));
  }

  .column {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    height: 100%;
  }

  .head,
  .foot {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
    padding: var(--space-2);
  }

  .head {
    padding: var(--space-0);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    flex: 1;
    min-height: 0;
    padding: var(--space-2);
    overflow-y: auto;
    overflow-x: hidden;
    mask-image: linear-gradient(to bottom, transparent, black var(--space-3), black calc(100% - var(--fade-size)), transparent);
  }

  .scrim {
    display: none;
  }

  @media (max-width: 56rem) {
    .sidebar {
      position: fixed;
      inset-block: 0;
      inset-inline-start: 0;
      width: calc(var(--sidebar-width) + var(--rail-space));
      transform: translateX(-100%);
      visibility: hidden;
      transition:
        transform var(--duration-slow) var(--ease-in-out),
        visibility var(--duration-slow) var(--ease-in-out);
    }

    .drawer {
      transform: none;
      visibility: visible;
      box-shadow: var(--ring-strong), inset calc(var(--border-width) * -1) 0 0 var(--color-border-subtle);
    }

    .scrim {
      display: block;
      position: fixed;
      inset: 0;
      z-index: var(--z-sidebar);
      background: var(--color-overlay);
      cursor: default;
    }
  }
</style>
