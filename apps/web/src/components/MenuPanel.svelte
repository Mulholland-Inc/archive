<script>
  // The floating surface every menu and option list draws on. `kind` says which, and so how it is sized and
  // capped: a dropdown (Menu) and a submenu (SubMenu) are menus, a context menu (ContextMenu) sits at x and y,
  // the pointer, and grows from its origin corner, a listbox (Select) lists options under its trigger, and a
  // picker (PeoplePicker) matches its field's width and leaves focus in the field.
  let { panel = $bindable(), kind = 'dropdown', width = 'md', popover = 'auto', label, x, y, origin, children, ...rest } = $props();
  const classes = $derived(['panel', 'floating', kind, `w-${width}`]);
</script>

{#if kind === 'context'}
  <div bind:this={panel} {popover} role="menu" tabindex="-1" aria-label={label} class={classes} style:--menu-x={`${x}px`} style:--menu-y={`${y}px`} style:--menu-origin={origin} {...rest}>
    {@render children()}
  </div>
{:else if kind === 'listbox'}
  <div bind:this={panel} {popover} role="listbox" tabindex="-1" aria-label={label} class={classes} {...rest}>
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
  .sub,
  .listbox {
    min-width: var(--menu-min);
  }

  .dropdown,
  .context,
  .sub {
    max-width: var(--menu-max);
  }

  .dropdown,
  .context,
  .listbox,
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

  .listbox {
    max-height: var(--listbox-max-height);
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
