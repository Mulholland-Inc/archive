<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let {
    variant = 'secondary',
    icon,
    trailing,
    disabled = false,
    pressed = false,
    onclick,
    children
  }: {
    variant?: 'secondary' | 'ghost' | 'tint';
    icon?: string;
    trailing?: string;
    disabled?: boolean;
    pressed?: boolean;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  } = $props();
</script>

<button class="button" data-variant={variant} data-pressed={pressed || undefined} aria-pressed={pressed || undefined} {disabled} {onclick}>
  {#if icon}<Icon name={icon} />{/if}
  {#if children}<span>{@render children()}</span>{/if}
  {#if trailing}<span class="trail"><Icon name={trailing} size="sm" /></span>{/if}
</button>

<style>
  .button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-1-5);
    flex: none;
    height: var(--control-height);
    padding: 0 var(--space-2-5);
    border-radius: var(--radius);
    font-size: var(--text-base);
    white-space: nowrap;
    color: var(--secondary-foreground);
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .button:active:not(:disabled) {
    transform: scale(var(--press-scale));
    transition-duration: var(--duration-instant);
  }
  .button:disabled {
    cursor: default;
    opacity: var(--opacity-disabled);
  }
  [data-variant='secondary'] {
    background: var(--secondary);
  }
  [data-variant='secondary']:hover:not(:disabled) {
    background: color-mix(in srgb, var(--secondary) 88%, var(--foreground));
  }
  [data-variant='ghost']:hover:not(:disabled),
  [data-variant='ghost'][data-pressed] {
    background: var(--hover);
  }
  [data-variant='tint'] {
    background: var(--tint-2);
  }
  [data-variant='tint']:hover:not(:disabled),
  [data-variant='tint'][data-pressed] {
    background: var(--tint-3);
  }
  .trail {
    display: grid;
    color: var(--muted-foreground);
    margin-left: calc(var(--space-0-5) * -1);
  }
</style>
