<script>
  let { value = $bindable(''), rows = 4, invalid = false, autosize = false, element = $bindable(), ...rest } = $props();
</script>

<textarea bind:this={element} bind:value {rows} class={['ta', { invalid, autosize }]} aria-invalid={invalid || undefined} {...rest}></textarea>

<style>
  .ta {
    display: block;
    width: 100%;
    min-height: var(--textarea-min);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    color: var(--color-text);
    line-height: var(--leading-normal);
    resize: vertical;
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .ta:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .ta:focus-visible {
    outline: none;
    box-shadow:
      inset 0 0 0 var(--border-width) var(--color-accent),
      0 0 0 calc(var(--focus-width) + var(--border-width)) var(--color-accent-subtle);
  }

  .ta::placeholder {
    color: var(--color-text-tertiary);
  }

  .invalid {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-danger);
  }

  .autosize {
    field-sizing: content;
    resize: none;
  }
</style>
