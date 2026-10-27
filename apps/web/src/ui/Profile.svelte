<script>
  import Avatar from './Avatar.svelte';
  import Menu from './Menu.svelte';

  let { name, items, value = $bindable('') } = $props();
  let trigger = $state();
  let pop = $state();
  let open = $state(false);
  let index = $state(0);
  let at = $state({ x: 0, y: 0 });
  const current = $derived(items.find((i) => i.value === value));
  const marked = $derived(items.map((i) => ({ ...i, checked: i.value === value })));

  function toggle() {
    if (open) return pop.hidePopover();
    const r = trigger.getBoundingClientRect();
    at = { x: r.left, y: innerHeight - r.top };
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

<button bind:this={trigger} class="profile" class:open aria-haspopup="listbox" aria-expanded={open} aria-label="{name}, {current?.label ?? ''}" onclick={toggle} onkeydown={key}>
  <Avatar {name} />
  <span class="text">
    <span class="name">{name}</span>
    <span class="space">{current?.label ?? ''}</span>
  </span>
</button>
<div bind:this={pop} class="pop" popover="auto" ontoggle={(e) => (open = e.newState === 'open')} style:--x="{at.x}px" style:--y="{at.y}px">
  <Menu title="Spaces" items={marked} {index} onpick={pick} onhover={(n) => (index = n)} />
</div>

<style>
  .profile {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
    padding: var(--space-3);
    border: 0;
    border-radius: var(--radius-lg);
    background: none;
    color: var(--text);
    text-align: left;
    cursor: pointer;
    transition: background var(--dur) var(--ease);
  }
  .profile:hover, .profile:focus-visible, .open { outline: 0; background: var(--hover); }
  .text { display: flex; flex-direction: column; gap: var(--space-1); min-width: 0; line-height: 1; }
  .name, .space { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name { font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .space { font-size: var(--text-xs); color: var(--text-3); text-transform: capitalize; }
  .pop { position: fixed; inset: auto; left: var(--x); bottom: calc(var(--y) + var(--space-2)); margin: 0; padding: 0; border: 0; background: none; overflow: visible; }
  .pop :global(.label) { text-transform: capitalize; }
  @media (max-width: 767px) {
    .profile { flex: none; padding: var(--space-2); border-radius: var(--radius-pill); }
    .text { display: none; }
    .pop { left: auto; right: var(--space-3); }
  }
</style>
