<script>
  import Icon from './Icon.svelte';
  import Spinner from './Spinner.svelte';

  let {
    variant = 'secondary',
    size = 'md',
    icon,
    iconEnd,
    href,
    loading = false,
    disabled = false,
    full = false,
    type = 'button',
    children,
    ...rest
  } = $props();

  const iconSize = $derived(size === 'sm' ? 'sm' : 'md');
</script>

{#snippet content()}
  {#if loading}
    <Spinner size="sm" />
  {:else if icon}
    <Icon {icon} size={iconSize} />
  {/if}
  {#if children}
    <span class="label">{@render children()}</span>
  {/if}
  {#if iconEnd}
    <Icon icon={iconEnd} size="sm" />
  {/if}
{/snippet}

{#if href}
  <a {href} class={['btn', variant, size, { full }]} {...rest}>{@render content()}</a>
{:else}
  <button {type} class={['btn', variant, size, { full }]} disabled={disabled || loading} aria-busy={loading || undefined} {...rest}>
    {@render content()}
  </button>
{/if}

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-1);
    flex: none;
    width: fit-content;
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    line-height: var(--leading-none);
    white-space: nowrap;
    user-select: none;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      background-image var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      box-shadow var(--duration-fast) var(--ease-out),
      opacity var(--duration-fast) var(--ease-out),
      transform var(--duration-instant) var(--ease-out);
  }

  .btn:active:not(:disabled) {
    transform: scale(var(--scale-press));
  }

  .btn:disabled:not([aria-busy]) {
    opacity: var(--opacity-disabled);
  }

  .label {
    padding-inline: var(--space-1);
  }

  .sm {
    height: var(--control-sm);
    padding: var(--space-1-5);
    font-size: var(--text-sm);
    border-radius: var(--radius-sm);
  }

  .sm .label {
    padding-inline: var(--space-0-5);
  }

  .lg {
    height: var(--control-xl);
    padding: var(--space-3);
    font-size: var(--text-md);
    border-radius: var(--radius-lg);
  }

  .full {
    width: 100%;
  }

  .primary {
    background: var(--color-primary);
    color: var(--color-on-primary);
  }

  .primary:hover:not(:disabled) {
    background: var(--color-primary-hover);
  }

  .inverse {
    background: var(--color-inverse);
    color: var(--color-text-inverse);
  }

  .inverse:hover:not(:disabled) {
    background: var(--color-inverse-hover);
  }

  .secondary {
    background-color: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .secondary:hover:not(:disabled) {
    background-image: linear-gradient(var(--color-fill-subtle), var(--color-fill-subtle));
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .ghost {
    color: var(--color-text-secondary);
  }

  .ghost:hover:not(:disabled) {
    background: var(--color-fill);
    color: var(--color-text);
  }

  .subtle {
    background: var(--color-fill);
  }

  .subtle:hover:not(:disabled) {
    background: var(--color-fill-strong);
  }

  .accent {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
  }

  .accent:hover:not(:disabled) {
    background: var(--color-accent-hover);
  }

  .danger {
    background: var(--color-danger);
    color: var(--color-danger-contrast);
  }

  .danger:hover:not(:disabled) {
    background: var(--color-danger);
  }

  .link {
    height: auto;
    padding: var(--space-0);
    color: var(--color-accent);
    border-radius: var(--radius-xs);
  }

  .link .label {
    padding-inline: var(--space-0);
  }

  .link:hover:not(:disabled) {
    text-decoration: underline;
    text-underline-offset: var(--space-0-5);
  }
</style>
