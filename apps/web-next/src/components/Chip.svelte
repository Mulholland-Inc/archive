<script>
  import Icon from './Icon.svelte';
  import { X } from '@lucide/svelte';

  let { icon, selected = false, onclick, onremove, size = 'md', href, children } = $props();
  const interactive = $derived(Boolean(onclick || href));
</script>

{#snippet inner()}
  {#if icon}<Icon {icon} size="sm" />{/if}
  <span class="label">{@render children()}</span>
{/snippet}

<span class={['chip', size, { selected, interactive, removable: onremove }]}>
  {#if href}
    <a class="hit" {href}>{@render inner()}</a>
  {:else if onclick}
    <button type="button" class="hit" aria-pressed={selected} {onclick}>{@render inner()}</button>
  {:else}
    <span class="hit">{@render inner()}</span>
  {/if}
  {#if onremove}
    <button type="button" class="remove" aria-label="Remove" onclick={onremove}><Icon icon={X} size="xs" /></button>
  {/if}
</span>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    flex: none;
    width: fit-content;
    height: var(--control-md);
    border-radius: var(--radius-md);
    background: var(--color-fill);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    line-height: var(--leading-none);
    white-space: nowrap;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      transform var(--duration-instant) var(--ease-out);
  }

  .hit {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    height: 100%;
    padding: var(--space-2);
    border-radius: inherit;
  }

  .label {
    padding-inline: var(--space-0-5);
  }

  .sm {
    height: var(--control-xs);
    font-size: var(--text-xs);
    border-radius: var(--radius-sm);
  }

  .sm .hit {
    padding: var(--space-1);
    gap: var(--space-1);
  }

  .interactive:hover {
    background: var(--color-fill-strong);
    color: var(--color-text);
  }

  .interactive:active {
    transform: scale(var(--scale-press));
  }

  .selected {
    background: var(--color-inverse);
    color: var(--color-text-inverse);
  }

  .selected.interactive:hover {
    background: var(--color-inverse-hover);
    color: var(--color-text-inverse);
  }

  .removable .hit {
    padding-inline-end: var(--space-0);
  }

  .remove {
    display: grid;
    place-items: center;
    width: var(--control-xs);
    height: var(--control-xs);
    border-radius: var(--radius-sm);
    color: var(--color-text-tertiary);
  }

  .remove:hover {
    color: var(--color-text);
    background: var(--color-fill);
  }
</style>
