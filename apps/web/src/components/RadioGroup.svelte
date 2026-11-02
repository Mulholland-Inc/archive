<script>
  import { uid } from '../lib/ids.js';

  let { value = $bindable(), options = [], label, name = uid('radio'), direction = 'column' } = $props();
</script>

<div class={['group', direction]} role="radiogroup" aria-label={label}>
  {#each options as option (option.value)}
    <label class={['radio', { disabled: option.disabled }]}>
      <input type="radio" {name} value={option.value} bind:group={value} disabled={option.disabled} />
      <span>{option.label}</span>
    </label>
  {/each}
</div>

<style>
  .group {
    display: flex;
    gap: var(--space-3);
  }

  .column {
    flex-direction: column;
  }

  .row {
    flex-direction: row;
    gap: var(--space-5);
  }

  .radio {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    cursor: pointer;
    font-size: var(--text-md);
    user-select: none;
  }

  .disabled {
    opacity: var(--opacity-disabled-field);
    cursor: default;
  }

  input {
    appearance: none;
    flex: none;
    width: var(--check-size);
    height: var(--check-size);
    margin: 0;
    border-radius: var(--radius-full);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
    cursor: inherit;
    transition: box-shadow var(--duration-fast) var(--ease-emphasized);
  }

  input:hover:not(:disabled) {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-text-tertiary);
  }

  input:checked {
    box-shadow: inset 0 0 0 calc(var(--space-1) + var(--border-width)) var(--color-accent);
  }
</style>
