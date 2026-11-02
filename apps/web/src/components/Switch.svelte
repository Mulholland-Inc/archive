<script>
  let { checked = $bindable(false), disabled = false, label, size = 'md', visual = false, onchange, ...rest } = $props();

  function toggle() {
    checked = !checked;
    onchange?.(checked);
  }
</script>

{#if visual}
  <span class={['switch', size]} aria-hidden="true" data-on={checked}><span class="knob"></span></span>
{:else}
  <button type="button" role="switch" aria-checked={checked} aria-label={label} class={['switch', size]} {disabled} onclick={toggle} {...rest}>
    <span class="knob"></span>
  </button>
{/if}

<style>
  .switch {
    position: relative;
    display: inline-flex;
    align-items: center;
    flex: none;
    width: var(--switch-width);
    height: var(--switch-height);
    padding: var(--space-0-5);
    border-radius: var(--radius-full);
    background: var(--color-fill-strong);
    transition: background-color var(--duration-normal) var(--ease-out);
  }

  .switch:hover:not(:disabled) {
    background: var(--color-border-strong);
  }

  .switch[aria-checked='true'],
  .switch[data-on='true'] {
    background: var(--color-accent);
  }

  .switch[aria-checked='true']:hover:not(:disabled) {
    background: var(--color-accent-hover);
  }

  .switch:disabled {
    opacity: var(--opacity-disabled);
  }

  .knob {
    width: var(--switch-knob);
    height: var(--switch-knob);
    border-radius: var(--radius-full);
    background: var(--color-knob);
    box-shadow: var(--ring-subtle);
    transition: transform var(--duration-normal) var(--ease-emphasized);
  }

  .switch[aria-checked='true'] .knob,
  .switch[data-on='true'] .knob {
    transform: translateX(calc(var(--switch-width) - var(--switch-knob) - var(--space-1)));
  }
</style>
