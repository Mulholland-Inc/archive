<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  import { intercept } from '../router.svelte';
  let { href, icon, title, meta, children }: { href?: string; icon: string; title: string; meta?: string; children?: Snippet } = $props();
</script>

{#snippet body()}
  <span class="icon"><Icon name={icon} /></span>
  <span class="text">
    <span class="head"><span class="title">{title}</span>{#if meta}<span class="meta">{meta}</span>{/if}</span>
    {#if children}<span class="detail">{@render children()}</span>{/if}
  </span>
{/snippet}

{#if href}
  <a class="row" {href} onclick={(e) => intercept(e, href)}>{@render body()}</a>
{:else}
  <div class="row">{@render body()}</div>
{/if}

<style>
  .row {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    color: inherit;
    text-decoration: none;
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  a.row:hover,
  a.row:focus-visible {
    background: var(--hover);
  }
  .icon {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--space-8);
    height: var(--space-8);
    border-radius: var(--radius-md);
    background: var(--card);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
    color: var(--muted-foreground);
  }
  .text {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    min-width: 0;
    padding-top: var(--space-1);
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    min-width: 0;
  }
  .title {
    font-weight: var(--font-medium);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    flex: none;
    font-size: var(--text-sm);
    color: var(--muted-foreground);
  }
  .detail {
    font-size: var(--text-sm);
  }
</style>
