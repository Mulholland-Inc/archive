<script>
  import { setContext } from 'svelte';
  import MenuPanel from './MenuPanel.svelte';
  import { uid } from '../lib/ids.js';
  import { show, hide, roving, focusables } from '../lib/floating.js';

  // With `morph` the panel forms around the trigger instead of beside it: the trigger is drawn again inside the
  // panel, over the original, and the items grow away from it, above for a top placement and below otherwise.
  let { trigger, children, placement = 'bottom-start', width = 'md', morph = false, label, onopenchange } = $props();

  const id = uid('menu');
  const up = $derived(placement.startsWith('top'));
  let panel = $state();
  let origin = $state();
  let items = $state();
  let open = $state(false);
  let anchor = null;

  function form() {
    const r = origin.getBoundingClientRect();
    panel.style.setProperty('--x', `${r.left}px`);
    panel.style.setProperty('--w', `${r.width}px`);
    panel.style.setProperty('--top', `${r.top}px`);
    panel.style.setProperty('--bottom', `${innerHeight - r.bottom}px`);
    const rows = [...items.children];
    rows.forEach((el, i) => el.style.setProperty('--i', up ? rows.length - 1 - i : i));
  }

  function openAt(target, focus = morph && up ? 'last' : 'first') {
    anchor = target;
    if (morph) form();
    show(panel, anchor);
    requestAnimationFrame(() => {
      const items = focusables(panel);
      (focus === 'last' ? items.at(-1) : items.find((el) => el.getAttribute('aria-checked') === 'true' && !el.dataset.toggle) ?? items[0])?.focus();
    });
  }

  function close() {
    hide(panel);
  }

  function toggle(event) {
    if (open) close();
    else openAt(event.currentTarget);
  }

  function triggerKey(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      openAt(event.currentTarget, event.key === 'ArrowUp' ? 'last' : 'first');
    }
  }

  function ontoggle(event) {
    open = event.newState === 'open';
    onopenchange?.(open);
  }

  function onkeydown(event) {
    roving(event, panel, { onclose: close });
  }

  setContext('menu', { close });

  export function openMenu(target) {
    openAt(target);
  }

  const props = $derived({ onclick: toggle, onkeydown: triggerKey, 'aria-haspopup': 'menu', 'aria-expanded': open, 'aria-controls': id });
</script>

<svelte:window onresize={morph ? close : undefined} />

{#if morph}
  <div bind:this={origin} class="morph-origin" data-open={open || undefined}>{@render trigger?.(props)}</div>
  <MenuPanel bind:panel {id} {label} {width} data-morph={up ? 'up' : 'down'} {ontoggle} {onkeydown}>
    {#if !up}<div class="morph-head">{@render trigger?.(props)}</div>{/if}
    <div bind:this={items} class="morph-items">{@render children()}</div>
    {#if up}<div class="morph-head">{@render trigger?.(props)}</div>{/if}
  </MenuPanel>
{:else}
  {@render trigger?.(props)}
  <MenuPanel bind:panel {id} {label} {width} data-placement={placement} {ontoggle} {onkeydown}>
    {@render children()}
  </MenuPanel>
{/if}
