<script lang="ts">
  import type { Snippet } from 'svelte';
  let { header, footer, children }: { header?: Snippet; footer?: Snippet; children: Snippet } = $props();
</script>

<nav class="sidebar" aria-label="Main">
  {#if header}<div class="header">{@render header()}</div>{/if}
  <div class="body">{@render children()}</div>
  {#if footer}<div class="footer">{@render footer()}</div>{/if}
</nav>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    width: var(--sidebar-width);
    height: 100%;
    background: var(--sidebar-background);
  }
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: var(--topbar-height);
    padding: 0 var(--space-3) 0 var(--space-4);
  }
  .body {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    padding: var(--space-2) var(--space-3) var(--space-4) var(--space-4);
    overflow-y: auto;
  }
  .footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4) var(--space-4);
  }
</style>
