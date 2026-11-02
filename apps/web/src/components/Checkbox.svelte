<script>
  import { Check, Minus } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { checked = $bindable(false), indeterminate = false, disabled = false, label, children, ...rest } = $props();
</script>

<label class={['check', { disabled }]}>
  <span class="box">
    <input type="checkbox" bind:checked {indeterminate} {disabled} aria-label={label} {...rest} />
    <span class="mark" aria-hidden="true">
      {#if indeterminate}<Icon icon={Minus} size="xs" />{:else}<Icon icon={Check} size="xs" />{/if}
    </span>
  </span>
  {#if children}<span class="text">{@render children()}</span>{/if}
</label>

<style>
  .check {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    cursor: pointer;
    font-size: var(--text-md);
    user-select: none;
  }

  .disabled {
    cursor: default;
    opacity: var(--opacity-disabled-field);
  }

  .box {
    position: relative;
    display: inline-grid;
    flex: none;
    width: var(--check-size);
    height: var(--check-size);
  }

  input {
    appearance: none;
    width: 100%;
    height: 100%;
    margin: 0;
    border-radius: var(--radius-xs);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
    cursor: inherit;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      box-shadow var(--duration-fast) var(--ease-out);
  }

  input:hover:not(:disabled) {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-text-tertiary);
  }

  input:checked,
  input:indeterminate {
    background: var(--color-accent);
    box-shadow: none;
  }

  .mark {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    color: var(--color-accent-contrast);
    pointer-events: none;
    opacity: 0;
    transform: scale(var(--scale-check-hidden));
    transition:
      opacity var(--duration-fast) var(--ease-out),
      transform var(--duration-fast) var(--ease-emphasized);
  }

  .mark :global(svg) {
    stroke-width: var(--icon-stroke-bold);
  }

  input:checked + .mark,
  input:indeterminate + .mark {
    opacity: 1;
    transform: none;
  }
</style>
