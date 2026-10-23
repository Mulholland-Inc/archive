<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let { title, open = $bindable(true), children }: { title?: string; open?: boolean; children: Snippet } = $props();
</script>

<section class="section">
  {#if title}
    <button class="title" aria-expanded={open} onclick={() => (open = !open)}>
      {title}
      <span class="chev" data-open={open || undefined}><Icon name="chevron-down" size="sm" /></span>
    </button>
  {/if}
  <div class="collapse" data-open={open || undefined}>
    <div class="inner">{@render children()}</div>
  </div>
</section>

<style>
  .section {
    display: flex;
    flex-direction: column;
  }
  .title {
    display: inline-flex;
    align-items: center;
    align-self: flex-start;
    gap: var(--space-1-5);
    height: var(--sidebar-item-height);
    margin-bottom: var(--space-1);
    padding: 0 var(--space-0-5);
    font-size: var(--text-base);
    color: var(--foreground);
  }
  .chev {
    display: grid;
    transition: transform var(--duration-normal) var(--ease-spring);
  }
  .chev:not([data-open]) {
    transform: rotate(-90deg);
  }
  .collapse {
    display: grid;
    grid-template-rows: 0fr;
    opacity: 0;
    transition:
      grid-template-rows var(--duration-slow) var(--ease-out),
      opacity var(--duration-normal) var(--ease-out);
  }
  .collapse[data-open] {
    grid-template-rows: 1fr;
    opacity: 1;
  }
  .inner {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-1-5);
    min-height: 0;
    overflow: hidden;
    padding: var(--space-0-5);
    margin: calc(var(--space-0-5) * -1);
  }
</style>
