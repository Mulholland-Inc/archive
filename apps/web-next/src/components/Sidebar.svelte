<script>
  import { setContext } from 'svelte';

  let { collapsed = false, drawer = false, header, footer, children, onclose } = $props();

  setContext('sidebar', {
    get collapsed() {
      return collapsed && !drawer;
    }
  });
</script>

{#if drawer}
  <button type="button" class="scrim" aria-label="Close sidebar" tabindex="-1" onclick={onclose}></button>
{/if}
<aside class={['sidebar', { collapsed, drawer }]} aria-label="Sidebar">
  <div class="head">{@render header?.()}</div>
  <nav class="body">{@render children()}</nav>
  <div class="foot">{@render footer?.()}</div>
</aside>

<style>
  .sidebar {
    position: relative;
    z-index: var(--z-sidebar);
    display: flex;
    flex-direction: column;
    flex: none;
    width: var(--sidebar-width);
    height: 100%;
    background: var(--color-sidebar);
    transition:
      width var(--duration-slow) var(--ease-in-out),
      transform var(--duration-slow) var(--ease-in-out),
      background-color var(--duration-normal) var(--ease-out);
    overflow: hidden;
  }

  .collapsed {
    width: var(--sidebar-rail);
  }

  .head,
  .foot {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
    padding: var(--space-2);
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

  .foot {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .scrim {
    display: none;
  }

  @media (max-width: 56rem) {
    .sidebar {
      position: fixed;
      inset-block: 0;
      inset-inline-start: 0;
      width: var(--sidebar-width);
      transform: translateX(-100%);
      box-shadow: none;
    }

    .drawer {
      transform: none;
      box-shadow: var(--shadow-lg);
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
