<script>
  let { value = $bindable(), items = [], label, variant = 'line', onchange } = $props();

  function pick(v) {
    value = v;
    onchange?.(v);
  }

  function onkeydown(event) {
    const i = items.findIndex((t) => t.value === value);
    let next = i;
    if (event.key === 'ArrowRight') next = (i + 1) % items.length;
    else if (event.key === 'ArrowLeft') next = (i - 1 + items.length) % items.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = items.length - 1;
    else return;
    event.preventDefault();
    pick(items[next].value);
    event.currentTarget.querySelectorAll('[role="tab"]')[next]?.focus();
  }
</script>

<div class={['tabs', variant]} role="tablist" aria-label={label} tabindex="-1" {onkeydown}>
  {#each items as item (item.value)}
    <button
      type="button"
      role="tab"
      aria-selected={value === item.value}
      tabindex={value === item.value ? 0 : -1}
      class={['tab', { on: value === item.value }]}
      onclick={() => pick(item.value)}
    >
      <span>{item.label}</span>
      {#if item.count !== undefined}<span class="count">{item.count}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: var(--space-1);
    min-width: 0;
    overflow-x: auto;
    overscroll-behavior-y: auto;
  }

  .line {
    gap: var(--space-5);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .tab {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    flex: none;
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    line-height: var(--leading-none);
    transition: color var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
  }

  .line .tab {
    height: var(--control-xl);
  }

  .line .tab::after {
    content: '';
    position: absolute;
    inset-inline: 0;
    inset-block-end: 0;
    height: var(--focus-width);
    border-radius: var(--radius-full);
    background: var(--color-text);
    transform: scaleX(0);
    transition: transform var(--duration-normal) var(--ease-out);
  }

  .line .on::after {
    transform: scaleX(1);
  }

  .pill .tab {
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
  }

  .pill .tab span:first-child {
    padding-inline: var(--space-1);
  }

  .pill .on {
    background: var(--color-fill);
  }

  .tab:hover,
  .on {
    color: var(--color-text);
  }

  .count {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }
</style>
