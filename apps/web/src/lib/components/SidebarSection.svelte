<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let {
    title,
    open = $bindable(true),
    action,
    children
  }: { title?: string; open?: boolean; action?: Snippet; children: Snippet } = $props();
</script>

<section class="section">
  {#if title}
    <div class="head">
      <button class="title" aria-expanded={open} onclick={() => (open = !open)}>
        {title}
        <span class="chev" data-open={open || undefined}><Icon name="chevron-down" size="sm" /></span>
      </button>
      {#if action}<span class="action">{@render action()}</span>{/if}
    </div>
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
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-1);
  }
  .action {
    display: flex;
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }
  .section:hover .action,
  .action:focus-within {
    opacity: 1;
  }
  .title {
    display: inline-flex;
    align-items: center;
    align-self: flex-start;
    gap: var(--space-1-5);
    height: var(--sidebar-item-height);
    padding: 0 var(--space-0-5);
    font-size: var(--text-sm);
    color: var(--muted-foreground);
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
    align-items: stretch;
    gap: var(--space-0-5);
    min-height: 0;
    overflow: hidden;
    padding: var(--space-0-5);
    margin: calc(var(--space-0-5) * -1);
  }
  @media (hover: none) {
    .action {
      opacity: 1;
    }
  }
</style>
