<script>
  import { workspace } from '../api.js';
  import Bot from './Bot.svelte';

  let { title, children } = $props();
</script>

<div class="empty">
  <span class="who"><Bot seed={workspace()} size={110} bare /></span>
  <p class="title">{title}</p>
  {#if children}<p class="hint">{@render children()}</p>{/if}
</div>

<style>
  .empty {
    display: flex;
    flex: 1;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    min-height: 60svh;
    padding: var(--space-4);
    text-align: center;
  }
  .who { margin-bottom: var(--space-2); }
  .title { color: var(--fg); font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); }
  .hint { color: var(--fg-muted); }
</style>
