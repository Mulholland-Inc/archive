<script>
  import Icon from './Icon.svelte';
  import Menu from './Menu.svelte';

  let { label, items, value = $bindable(''), icon = 'chevron-down' } = $props();
  let trigger = $state();
  let pop = $state();
  let open = $state(false);
  let index = $state(0);
  let at = $state({ x: 0, y: 0 });
  const current = $derived(items.find((i) => i.value === value));

  function toggle() {
    if (open) return pop.hidePopover();
    const r = trigger.getBoundingClientRect();
    at = { x: r.left, y: r.bottom };
    index = Math.max(0, items.indexOf(current));
    pop.showPopover();
  }

  function pick(item) {
    value = item.value;
    pop.hidePopover();
  }

  function key(e) {
    if (!open) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      index = (index + (e.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length;
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      pick(items[index]);
    }
  }
</script>

<button bind:this={trigger} class="select" class:set={value !== ''} aria-haspopup="listbox" aria-expanded={open} onclick={toggle} onkeydown={key}>
  <span>{current && value !== '' ? `${label}: ${current.label}` : label}</span>
  <Icon name={icon} />
</button>
<div bind:this={pop} class="pop" popover="auto" ontoggle={(e) => (open = e.newState === 'open')} style:--x="{at.x}px" style:--y="{at.y}px">
  <Menu compact items={items} {index} onpick={pick} onhover={(n) => (index = n)} />
</div>

<style>
  .select {
    display: inline-flex;
    flex: none;
    align-items: center;
    gap: var(--space-2);
    height: var(--size-sm);
    padding: var(--space-3);
    border: 0;
    border-radius: var(--radius-pill);
    background: var(--panel);
    color: var(--text-2);
    font-size: var(--text-sm);
    line-height: 1;
    white-space: nowrap;
    text-transform: capitalize;
    cursor: pointer;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .select:hover, .select:focus-visible, .select[aria-expanded='true'] { outline: 0; background: var(--raised); color: var(--text); }
  .set, .set:hover, .set:focus-visible, .set[aria-expanded='true'] { background: var(--cream); color: var(--cream-ink); }
  .set:hover, .set:focus-visible { background: var(--cream-hover); }
  .select :global(svg) { width: var(--text-md); height: var(--text-md); }
  .pop { position: fixed; inset: auto; left: var(--x); top: calc(var(--y) + var(--space-2)); margin: 0; padding: 0; border: 0; background: none; overflow: visible; text-transform: capitalize; }
</style>
