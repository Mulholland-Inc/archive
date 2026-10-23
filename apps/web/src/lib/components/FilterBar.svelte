<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let {
    query = $bindable(''),
    placeholder = 'Ask or filter…',
    focused = false,
    onfocus,
    chips
  }: { query?: string; placeholder?: string; focused?: boolean; onfocus?: () => void; chips?: Snippet } = $props();
  let input: HTMLInputElement;
</script>

<div class="bar" data-focused={focused || undefined} role="search" onpointerdown={(e) => { if (e.target === e.currentTarget) { e.preventDefault(); input.focus(); } }}>
  <span class="sparkle"><Icon name="sparkle" size="lg" /></span>
  {#if chips}{@render chips()}{/if}
  <input bind:this={input} bind:value={query} {placeholder} aria-label="Filter records" onfocus={onfocus} />
</div>

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1-5);
    min-height: calc(var(--control-height) + var(--space-2));
    padding: var(--space-1) var(--space-1) var(--space-1) var(--space-2);
    border-radius: var(--radius-md);
    background: var(--surface);
    box-shadow: inset 0 0 0 var(--border-width-strong) var(--border);
    cursor: text;
    transition: box-shadow var(--duration-normal) var(--ease-out);
  }
  .bar:focus-within,
  .bar[data-focused] {
    box-shadow:
      inset 0 0 0 var(--border-width-strong) var(--ring-soft),
      0 0 0 calc(var(--focus-width) + var(--space-0-5)) color-mix(in srgb, var(--ring-soft) 45%, transparent);
  }
  .sparkle {
    display: grid;
    padding-right: var(--space-0-5);
    transition: transform var(--duration-slow) var(--ease-spring);
  }
  .bar:focus-within .sparkle {
    transform: rotate(var(--sparkle-rotate)) scale(var(--hover-scale));
  }
  input {
    flex: 1;
    min-width: calc(var(--space-12) * 2);
    height: var(--control-height);
    padding: 0 var(--space-1);
    caret-color: var(--foreground);
  }
  input::placeholder {
    color: var(--subtle-foreground);
  }
</style>
