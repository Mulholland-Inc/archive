<script lang="ts">
  import { intercept } from '../router.svelte';
  let {
    href,
    title,
    rows = [],
    state = 'rest'
  }: { href: string; title: string; rows?: { label: string; value: string }[]; state?: 'rest' | 'lifted' | 'placeholder' } = $props();
</script>

<a class="tile" {href} data-state={state} onclick={(e) => intercept(e, href)} draggable="false">
  <span class="title">{title}</span>
  {#each rows as row (row.label)}
    <span class="row"><span class="label">{row.label}</span><span class="value">{row.value}</span></span>
  {/each}
</a>

<style>
  .tile {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--card-padding);
    border-radius: var(--card-radius);
    background: var(--card);
    color: var(--card-foreground);
    text-decoration: none;
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
    user-select: none;
    -webkit-user-drag: none;
    transition:
      box-shadow var(--duration-normal) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring),
      opacity var(--duration-normal) var(--ease-out);
  }
  .tile:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--tint-4);
  }
  [data-state='placeholder'] {
    opacity: var(--opacity-placeholder);
  }
  [data-state='lifted'] {
    transform: scale(var(--lift-scale)) rotate(var(--lift-rotate));
    box-shadow: var(--shadow-lift);
  }
  .title {
    font-weight: var(--font-medium);
    overflow-wrap: anywhere;
  }
  .row {
    display: flex;
    gap: var(--space-2);
    min-width: 0;
    font-size: var(--text-sm);
  }
  .label {
    flex: none;
    color: var(--muted-foreground);
  }
  .value {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
