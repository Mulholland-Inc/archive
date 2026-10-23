<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    name,
    surface = 'surface',
    tall = false,
    children
  }: { name: string; surface?: 'surface' | 'card' | 'background'; tall?: boolean; children: Snippet } = $props();
</script>

<figure class="specimen">
  <div class="stage" data-surface={surface} data-tall={tall || undefined}>{@render children()}</div>
  <figcaption class="name">{name}</figcaption>
</figure>

<style>
  .specimen {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
  }
  .stage {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-3);
    min-height: calc(var(--space-20) + var(--space-4));
    padding: var(--space-5);
    border-radius: var(--radius-xl);
    background: var(--surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--tint-1);
  }
  [data-tall] {
    height: var(--specimen-height);
    padding: 0;
    overflow: hidden;
  }
  [data-surface='card'] {
    background: var(--card);
  }
  [data-surface='background'] {
    background: var(--background);
  }
  .name {
    padding: 0 var(--space-1);
    font-size: var(--text-sm);
    color: var(--muted-foreground);
  }
</style>
