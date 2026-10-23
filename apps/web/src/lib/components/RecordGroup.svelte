<script lang="ts">
  import type { Snippet } from 'svelte';
  import GroupPill from './GroupPill.svelte';
  import Count from './Count.svelte';
  import IconButton from './IconButton.svelte';
  let {
    status,
    glyph,
    label,
    count,
    collapsed = $bindable(false),
    onadd,
    children
  }: { status: string; glyph?: string; label: string; count: number; collapsed?: boolean; onadd?: () => void; children: Snippet } = $props();
</script>

<section class="group">
  <header class="header">
    <GroupPill {status} {glyph} {label} {collapsed} onclick={() => (collapsed = !collapsed)} />
    <Count value={count} />
    <IconButton icon="plus" label={`Add to ${label}`} onclick={onadd} />
  </header>
  <div class="collapse" data-open={!collapsed || undefined}>
    <div class="inner">{@render children()}</div>
  </div>
</section>

<style>
  .group {
    display: flex;
    flex-direction: column;
  }
  .header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding-bottom: var(--space-2);
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
    min-height: 0;
    overflow: hidden;
    padding: 0 var(--space-2) var(--space-6);
    margin: 0 calc(var(--space-2) * -1);
  }
</style>
