<script>
  import Icon from './Icon.svelte';

  let { value = $bindable(), options = [], size = 'md', label, iconOnly = false, full = false, onchange } = $props();

  function pick(v) {
    value = v;
    onchange?.(v);
  }

  function onkeydown(event) {
    const i = options.findIndex((o) => o.value === value);
    let next = i;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = (i + 1) % options.length;
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = (i - 1 + options.length) % options.length;
    else return;
    event.preventDefault();
    pick(options[next].value);
    event.currentTarget.querySelectorAll('button')[next]?.focus();
  }
</script>

<div class={['seg', size, { full }]} role="radiogroup" aria-label={label} tabindex="-1" {onkeydown}>
  {#each options as option (option.value)}
    <button
      type="button"
      role="radio"
      aria-checked={value === option.value}
      aria-label={iconOnly ? option.label : undefined}
      title={iconOnly ? option.label : undefined}
      tabindex={value === option.value ? 0 : -1}
      class={['item', { on: value === option.value }]}
      onclick={() => pick(option.value)}
    >
      {#if option.icon}<Icon icon={option.icon} size="sm" />{/if}
      {#if !iconOnly}<span>{option.label}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .seg {
    display: inline-flex;
    flex: none;
    width: fit-content;
    gap: var(--space-0-5);
    padding: var(--space-0-5);
    border-radius: var(--radius-md);
    background: var(--color-fill);
  }

  .full {
    display: flex;
    width: 100%;
  }

  .full .item {
    flex: 1;
  }

  .item {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-1-5);
    height: var(--control-sm);
    min-width: var(--control-sm);
    padding: var(--space-1-5);
    border-radius: var(--radius-sm);
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    line-height: var(--leading-none);
    white-space: nowrap;
    transition:
      background-color var(--duration-normal) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      box-shadow var(--duration-normal) var(--ease-out);
  }

  .item span {
    padding-inline: var(--space-1);
  }

  .item:hover:not(.on) {
    color: var(--color-text);
  }

  .on {
    background: var(--color-surface-raised);
    color: var(--color-text);
    box-shadow: var(--shadow-xs), 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .sm .item {
    height: var(--control-xs);
    min-width: var(--control-xs);
    padding: var(--space-1);
    font-size: var(--text-xs);
  }

  .lg .item {
    height: var(--control-md);
    padding: var(--space-2);
    font-size: var(--text-md);
  }
</style>
