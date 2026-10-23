<script lang="ts">
  import type { Snippet } from 'svelte';
  import FileThumb from './FileThumb.svelte';
  import { intercept } from '../router.svelte';
  let { href, src, mime, name, detail, status }: { href: string; src?: string; mime: string; name: string; detail?: string; status?: Snippet } = $props();
</script>

<a class="card" {href} onclick={(e) => intercept(e, href)}>
  <FileThumb {src} {mime} {name} />
  <span class="name" title={name}>{name}</span>
  <span class="meta">
    {#if detail}<span>{detail}</span>{/if}
    {#if status}{@render status()}{/if}
  </span>
</a>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-2);
    border-radius: var(--radius-lg);
    color: inherit;
    text-decoration: none;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
    animation: card-in var(--duration-slow) var(--ease-out);
  }
  .card:hover {
    background: var(--hover);
  }
  .card:active {
    transform: scale(var(--press-scale-subtle));
  }
  .name {
    padding: 0 var(--space-1);
    font-weight: var(--font-medium);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    display: flex;
    justify-content: space-between;
    gap: var(--space-2);
    padding: 0 var(--space-1);
    font-size: var(--text-sm);
    color: var(--muted-foreground);
  }
  @keyframes card-in {
    from {
      opacity: 0;
      transform: translateY(var(--nudge));
    }
  }
</style>
