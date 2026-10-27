<script>
  import Avatar from './Avatar.svelte';
  import Menu from './Menu.svelte';

  let { name, items, value = $bindable(''), onadd } = $props();
  let trigger = $state();
  let pop = $state();
  let open = $state(false);
  let index = $state(0);
  let adding = $state(false);
  let draft = $state('');
  let error = $state('');
  let at = $state({ x: 0, y: 0 });
  const current = $derived(items.find((i) => i.value === value));
  const options = $derived([...items.map((i) => ({ ...i, checked: i.value === value })), { label: 'Add space', after: 'plus', add: true }]);

  function toggle() {
    if (open) return pop.hidePopover();
    const r = trigger.getBoundingClientRect();
    at = { x: r.left, y: innerHeight - r.top };
    index = Math.max(0, items.indexOf(current));
    adding = false;
    draft = '';
    error = '';
    pop.showPopover();
  }

  function pick(item) {
    if (item.add) {
      adding = true;
      return;
    }
    value = item.value;
    pop.hidePopover();
  }

  async function create(e) {
    e.preventDefault();
    if (!draft.trim()) return;
    try {
      await onadd(draft.trim());
      pop.hidePopover();
    } catch (err) {
      error = err.message;
    }
  }

  function key(e) {
    if (!open || adding) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      index = (index + (e.key === 'ArrowDown' ? 1 : options.length - 1)) % options.length;
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      pick(options[index]);
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
  {#if adding}
    <form class="add" onsubmit={create}>
      <input bind:value={draft} placeholder="Space name" aria-label="Space name" {@attach (el) => el.focus()} onkeydown={(e) => e.key === 'Escape' && (adding = false)} />
      {#if error}<span class="error">{error}</span>{/if}
    </form>
  {:else}
    <Menu items={options} {index} onpick={pick} onhover={(n) => (index = n)} />
  {/if}
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
    border-radius: var(--radius-pill);
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
  .space { font-size: var(--text-xs); color: var(--text-3); }
  .pop { position: fixed; inset: auto; left: var(--x); bottom: calc(var(--y) + var(--space-2)); margin: 0; padding: 0; border: 0; background: none; overflow: visible; }
  .add { display: flex; flex-direction: column; gap: var(--space-2); width: var(--menu); padding: var(--space-2); background: var(--raised); border-radius: var(--radius-lg); box-shadow: var(--shadow); }
  .add input { padding: var(--space-3); border: 0; border-radius: var(--radius-sm); background: var(--panel); font-size: var(--text-sm); outline: 0; }
  .add input::placeholder { color: var(--text-3); }
  .error { padding: var(--space-2); font-size: var(--text-xs); color: var(--alarm); }
  @media (max-width: 767px) {
    .profile { flex: none; padding: var(--space-2); border-radius: var(--radius-pill); }
    .text { display: none; }
    .pop { left: auto; right: var(--space-3); }
  }
</style>
