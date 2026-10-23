<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    collapsed = false,
    embedded = false,
    onclose,
    sidebar,
    topbar,
    children
  }: { collapsed?: boolean; embedded?: boolean; onclose?: () => void; sidebar: Snippet; topbar: Snippet; children: Snippet } = $props();
</script>

<div class="shell" data-collapsed={collapsed || undefined} data-embedded={embedded || undefined}>
  <aside class="side" aria-hidden={collapsed || undefined} inert={collapsed}>
    <div class="side-inner">{@render sidebar()}</div>
  </aside>
  <button class="scrim" aria-label="Close menu" tabindex="-1" onclick={onclose}></button>
  <div class="main">
    <header class="topbar">{@render topbar()}</header>
    <main class="panel">{@render children()}</main>
  </div>
</div>

<style>
  .shell {
    position: relative;
    display: grid;
    grid-template-columns: var(--sidebar-width) minmax(0, 1fr);
    height: 100vh;
    height: 100dvh;
    background: var(--sidebar-background);
    transition: grid-template-columns var(--duration-slow) var(--ease-out);
  }
  [data-embedded] {
    height: 100%;
    width: 100%;
    border-radius: var(--radius-xl);
    overflow: hidden;
  }
  [data-collapsed] {
    grid-template-columns: 0 minmax(0, 1fr);
  }
  .side {
    grid-column: 1;
    grid-row: 1;
    min-width: 0;
    overflow: hidden;
  }
  .side-inner {
    width: var(--sidebar-width);
    height: 100%;
    transition:
      opacity var(--duration-normal) var(--ease-out),
      transform var(--duration-slow) var(--ease-out);
  }
  [data-collapsed] .side-inner {
    opacity: 0;
    transform: translateX(calc(var(--slide) * -1));
  }
  .scrim {
    display: none;
  }
  .main {
    grid-column: 2;
    grid-row: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .topbar {
    display: flex;
    align-items: center;
    height: var(--topbar-height);
    padding: 0 var(--space-4);
    flex: none;
    min-width: 0;
    overflow: hidden;
  }
  .panel {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border-top-left-radius: var(--panel-radius);
    overflow: hidden;
    transition: border-radius var(--duration-slow) var(--ease-out);
  }
  [data-collapsed] .panel {
    border-top-left-radius: 0;
  }

  @media (max-width: 64rem) {
    .shell:not([data-embedded]),
    .shell:not([data-embedded])[data-collapsed] {
      grid-template-columns: 0 minmax(0, 1fr);
    }
    .shell:not([data-embedded]) .side {
      position: fixed;
      top: 0;
      bottom: 0;
      left: 0;
      z-index: var(--z-dialog);
      width: min(var(--sidebar-width), 86vw);
      background: var(--sidebar-background);
      box-shadow: var(--shadow-lift);
      transition:
        transform var(--duration-slow) var(--ease-out),
        box-shadow var(--duration-slow) var(--ease-out);
    }
    .shell:not([data-embedded])[data-collapsed] .side {
      transform: translateX(-100%);
      box-shadow: none;
    }
    .shell:not([data-embedded]) .side-inner,
    .shell:not([data-embedded])[data-collapsed] .side-inner {
      width: 100%;
      opacity: 1;
      transform: none;
    }
    .shell:not([data-embedded]) .scrim {
      display: block;
      position: fixed;
      inset: 0;
      z-index: calc(var(--z-dialog) - 1);
      background: var(--overlay);
      opacity: 1;
      transition:
        opacity var(--duration-normal) var(--ease-out),
        visibility var(--duration-normal);
    }
    .shell:not([data-embedded])[data-collapsed] .scrim {
      opacity: 0;
      visibility: hidden;
    }
    .shell:not([data-embedded]) .panel {
      border-top-left-radius: 0;
    }
    .topbar {
      padding: 0 var(--space-2);
    }
  }
</style>
