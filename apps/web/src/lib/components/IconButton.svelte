<script lang="ts">
  import Icon from './Icon.svelte';
  let {
    icon,
    label,
    variant = 'ghost',
    size = 'md',
    pressed = false,
    disabled = false,
    onclick
  }: {
    icon: string;
    label: string;
    variant?: 'ghost' | 'solid' | 'tint';
    size?: 'sm' | 'md' | 'lg';
    pressed?: boolean;
    disabled?: boolean;
    onclick?: (e: MouseEvent) => void;
  } = $props();
</script>

<button
  class="icon-button"
  data-variant={variant}
  data-size={size}
  data-pressed={pressed || undefined}
  aria-label={label}
  title={label}
  aria-pressed={pressed || undefined}
  {disabled}
  {onclick}
>
  <Icon name={icon} size={size === 'lg' ? 'lg' : 'md'} />
</button>

<style>
  .icon-button {
    display: inline-grid;
    place-items: center;
    flex: none;
    width: var(--control-height);
    height: var(--control-height);
    border-radius: var(--radius);
    color: var(--foreground);
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .icon-button:active:not(:disabled) {
    transform: scale(var(--press-scale-icon));
    transition-duration: var(--duration-instant);
  }
  .icon-button:disabled {
    opacity: var(--opacity-disabled);
    cursor: default;
  }
  [data-variant='ghost']:hover:not(:disabled),
  [data-variant='ghost'][data-pressed] {
    background: var(--hover);
  }
  [data-variant='tint'] {
    background: var(--tint-2);
  }
  [data-variant='tint']:hover {
    background: var(--tint-3);
  }
  [data-variant='solid'] {
    background: var(--secondary);
    width: calc(var(--control-height) * 1.6);
    height: calc(var(--control-height) * 1.15);
  }
  [data-variant='solid']:hover {
    background: color-mix(in srgb, var(--secondary) 90%, var(--foreground));
  }
  [data-size='sm'] {
    width: calc(var(--control-height) - var(--space-1));
    height: calc(var(--control-height) - var(--space-1));
  }
</style>
