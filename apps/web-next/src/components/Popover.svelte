<script>
  import { uid } from '../lib/ids.js';
  import { show, hide } from '../lib/floating.js';

  let { trigger, children, placement = 'bottom-start', label, width = 'md', open = $bindable(false) } = $props();

  const id = uid('pop');
  let panel = $state();

  function toggle(event) {
    if (open) hide(panel);
    else show(panel, event.currentTarget);
  }

  export function close() {
    hide(panel);
  }
</script>

{@render trigger({ onclick: toggle, 'aria-haspopup': 'dialog', 'aria-expanded': open, 'aria-controls': id })}

<div
  bind:this={panel}
  {id}
  popover="auto"
  role="dialog"
  aria-label={label}
  class={['popover', 'floating', width]}
  data-placement={placement}
  ontoggle={(e) => (open = e.newState === 'open')}
>
  {@render children({ close: () => hide(panel) })}
</div>

<style>
  .popover {
    width: var(--popover-width);
    padding: var(--space-4);
    border-radius: var(--radius-xl);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
  }

  .sm {
    width: calc(var(--popover-width) * 0.75);
  }

  .lg {
    width: var(--dialog-sm);
  }
</style>
