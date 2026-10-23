<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    collapsed = false,
    embedded = false,
    sidebar,
    topbar,
    children
  }: { collapsed?: boolean; embedded?: boolean; sidebar: Snippet; topbar: Snippet; children: Snippet } = $props();
</script>

<div class="shell" data-collapsed={collapsed || undefined} data-embedded={embedded || undefined}>
  <aside class="side" aria-hidden={collapsed || undefined} inert={collapsed}>
    <div class="side-inner">{@render sidebar()}</div>
  </aside>
  <div class="main">
    <header class="topbar">{@render topbar()}</header>
    <main class="panel">{@render children()}</main>
  </div>
</div>

<style>
  .shell {
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
  .main {
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
</style>
