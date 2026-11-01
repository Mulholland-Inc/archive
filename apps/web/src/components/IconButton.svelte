<script>
  import Icon from './Icon.svelte';
  import Tooltip from './Tooltip.svelte';

  let {
    icon,
    label,
    size = 'md',
    variant = 'ghost',
    round = false,
    tooltip = true,
    placement = 'top',
    shortcut,
    pressed,
    href,
    disabled = false,
    ...rest
  } = $props();

  const iconSize = $derived(size === 'lg' ? 'lg' : size === 'xs' ? 'xs' : size === 'sm' ? 'sm' : 'md');
</script>

{#snippet control()}
  {#if href}
    <a {href} class={['ib', variant, size, { round, pressed }]} aria-label={label} {...rest}>
      <Icon {icon} size={iconSize} />
    </a>
  {:else}
    <button type="button" class={['ib', variant, size, { round, pressed }]} aria-label={label} aria-pressed={pressed} {disabled} {...rest}>
      <Icon {icon} size={iconSize} />
    </button>
  {/if}
{/snippet}

{#if tooltip && !disabled}
  <Tooltip text={label} {shortcut} {placement}>{@render control()}</Tooltip>
{:else}
  {@render control()}
{/if}

<style>
  .ib {
    display: inline-grid;
    place-items: center;
    flex: none;
    width: var(--control-md);
    height: var(--control-md);
    padding: var(--space-0);
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      box-shadow var(--duration-fast) var(--ease-out),
      opacity var(--duration-fast) var(--ease-out),
      transform var(--duration-instant) var(--ease-out);
  }

  .ib:active:not(:disabled) {
    transform: scale(var(--scale-press));
  }

  .ib:disabled {
    opacity: 0.4;
  }

  .xs {
    width: var(--control-xs);
    height: var(--control-xs);
    border-radius: var(--radius-sm);
  }

  .sm {
    width: var(--control-sm);
    height: var(--control-sm);
    border-radius: var(--radius-sm);
  }

  .lg {
    width: var(--control-lg);
    height: var(--control-lg);
  }

  .round {
    border-radius: var(--radius-full);
  }

  .ghost:hover:not(:disabled),
  .ghost.pressed {
    background: var(--color-fill);
    color: var(--color-text);
  }

  .subtle {
    background: var(--color-fill);
  }

  .subtle:hover:not(:disabled) {
    background: var(--color-fill-strong);
    color: var(--color-text);
  }

  .secondary {
    background-color: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .secondary:hover:not(:disabled) {
    background-image: linear-gradient(var(--color-fill-subtle), var(--color-fill-subtle));
    color: var(--color-text);
  }

  .accent {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
  }

  .accent:hover:not(:disabled) {
    background: var(--color-accent-hover);
  }

  .primary {
    background: var(--color-primary);
    color: var(--color-on-primary);
  }

  .primary:hover:not(:disabled) {
    background: var(--color-primary-hover);
  }

  .accent:disabled,
  .primary:disabled {
    background: var(--color-fill-strong);
    color: var(--color-text-tertiary);
    opacity: 1;
  }

  .inverse {
    background: var(--color-inverse);
    color: var(--color-text-inverse);
  }

  .inverse:hover:not(:disabled) {
    background: var(--color-inverse-hover);
  }
</style>
