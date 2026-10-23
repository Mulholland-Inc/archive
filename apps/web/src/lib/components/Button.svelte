<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let {
    variant = 'secondary',
    size = 'md',
    type = 'button',
    icon,
    trailing,
    loading = false,
    disabled = false,
    pressed = false,
    block = false,
    onclick,
    children
  }: {
    variant?: 'secondary' | 'ghost' | 'tint' | 'primary' | 'danger';
    size?: 'md' | 'lg';
    type?: 'button' | 'submit';
    icon?: string;
    trailing?: string;
    loading?: boolean;
    disabled?: boolean;
    pressed?: boolean;
    block?: boolean;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  } = $props();
</script>

<button
  class="button"
  {type}
  data-variant={variant}
  data-size={size}
  data-block={block || undefined}
  data-loading={loading || undefined}
  data-pressed={pressed || undefined}
  aria-pressed={pressed || undefined}
  aria-busy={loading || undefined}
  disabled={disabled || loading}
  {onclick}
>
  {#if loading}
    <span class="lead"><Icon name="spinner" /></span>
  {:else if icon}
    <span class="lead"><Icon name={icon} /></span>
  {/if}
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
      color var(--duration-fast) var(--ease-out),
      opacity var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .button:active:not(:disabled) {
    transform: scale(var(--press-scale));
    transition-duration: var(--duration-instant);
  }
  .button:disabled {
    cursor: default;
  }
  .button:disabled:not([data-loading]) {
    opacity: var(--opacity-disabled);
  }
  [data-size='lg'] {
    height: var(--field-height);
    padding: 0 var(--space-3-5);
    border-radius: var(--field-radius);
  }
  [data-block] {
    width: 100%;
  }
  [data-variant='secondary'] {
    background: var(--secondary);
    box-shadow: inset 0 0 0 var(--border-width) var(--tint-2);
  }
  [data-variant='secondary']:hover:not(:disabled),
  [data-variant='secondary'][data-pressed] {
    background: color-mix(in srgb, var(--secondary) 90%, var(--foreground));
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
  [data-variant='primary'] {
    background: var(--primary);
    color: var(--primary-foreground);
    font-weight: var(--font-medium);
  }
  [data-variant='primary']:hover:not(:disabled) {
    background: color-mix(in srgb, var(--primary) 84%, var(--primary-foreground));
  }
  [data-variant='danger'] {
    background: var(--danger);
    color: var(--destructive-foreground);
    font-weight: var(--font-medium);
  }
  [data-variant='danger']:hover:not(:disabled) {
    background: color-mix(in srgb, var(--danger) 88%, var(--ink));
  }
  .lead {
    display: grid;
  }
  .trail {
    display: grid;
    color: var(--muted-foreground);
    margin-left: calc(var(--space-0-5) * -1);
  }
  [data-variant='primary'] .trail,
  [data-variant='danger'] .trail {
    color: inherit;
  }
</style>
