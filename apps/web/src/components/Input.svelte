<script>
  import Icon from './Icon.svelte';

  let {
    value = $bindable(''),
    size = 'md',
    icon,
    trailing,
    invalid = false,
    disabled = false,
    element = $bindable(),
    type = 'text',
    ...rest
  } = $props();
</script>

<span class={['box', size, { invalid, disabled }]}>
  {#if icon}<Icon {icon} size="md" />{/if}
  <input bind:this={element} bind:value {type} {disabled} aria-invalid={invalid || undefined} {...rest} />
  {#if trailing}{@render trailing()}{/if}
</span>

<style>
  .box {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: var(--control-xl);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    color: var(--color-text-tertiary);
    transition:
      box-shadow var(--duration-fast) var(--ease-out),
      background-color var(--duration-fast) var(--ease-out);
    min-width: 0;
  }

  .box:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .box:focus-within {
    box-shadow:
      inset 0 0 0 var(--border-width) var(--color-accent),
      0 0 0 calc(var(--focus-width) + var(--border-width)) var(--color-accent-subtle);
  }

  .sm {
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    font-size: var(--text-md);
  }

  .lg {
    height: var(--control-2xl);
    padding: var(--space-4);
    font-size: var(--text-base);
  }

  .invalid,
  .invalid:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-danger);
  }

  .disabled {
    opacity: var(--opacity-disabled-input);
    background: var(--color-surface-sunken);
  }

  input {
    flex: 1;
    min-width: 0;
    color: var(--color-text);
    line-height: var(--leading-normal);
  }

  input::placeholder {
    color: var(--color-text-tertiary);
  }

  input:focus-visible {
    outline: none;
  }

  input[type='search']::-webkit-search-cancel-button {
    display: none;
  }
</style>
