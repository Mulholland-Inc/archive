<script>
  let { panel = $bindable(), kind = 'dropdown', width = 'md', popover = 'auto', label, x, y, origin, children, ...rest } = $props();
  const classes = $derived(['panel', 'floating', kind, `w-${width}`]);
</script>

{#if kind === 'context'}
  <div bind:this={panel} {popover} role="menu" tabindex="-1" aria-label={label} class={classes} style:--menu-x={`${x}px`} style:--menu-y={`${y}px`} style:--menu-origin={origin} {...rest}>
    {@render children()}
  </div>
{:else if kind === 'picker'}
  <div bind:this={panel} {popover} role="listbox" aria-label={label} class={classes} {...rest}>
    {@render children()}
  </div>
{:else}
  <div bind:this={panel} {popover} role="menu" tabindex="-1" aria-label={label} class={classes} {...rest}>
    {@render children()}
  </div>
{/if}

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
    padding: var(--space-1-5);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
  }

  .dropdown,
  .context,
  .sub {
    min-width: var(--menu-min);
  }

  .dropdown,
  .context,
  .sub {
    max-width: var(--menu-max);
  }

  .dropdown,
  .context,
  .picker {
    overflow-y: auto;
  }

  .dropdown,
  .context {
    font-size: var(--text-md);
    z-index: var(--z-popover);
  }

  .dropdown {
    max-height: var(--menu-max-height);
  }

  .context {
    inset-inline-start: var(--menu-x);
    inset-block-start: var(--menu-y);
    transform-origin: var(--menu-origin);
    max-height: calc(100vh - var(--space-4));
  }

  .picker {
    max-height: var(--picker-max-height);
  }

  .w-sm {
    min-width: var(--menu-min-sm);
  }

  .w-lg {
    min-width: var(--popover-width);
  }
</style>
