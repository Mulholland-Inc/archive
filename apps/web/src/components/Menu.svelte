<script>
  import { setContext } from 'svelte';
  import MenuPanel from './MenuPanel.svelte';
  import { uid } from '../lib/ids.js';
  import { show, hide, roving, focusables } from '../lib/floating.js';

  let { trigger, children, placement = 'bottom-start', width = 'md', match = false, label, onopenchange } = $props();

  const id = uid('menu');
  let panel = $state();
  let open = $state(false);
  let anchor = null;

  function openAt(target, focus = 'first') {
    anchor = target;
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
</script>

{@render trigger?.({ onclick: toggle, onkeydown: triggerKey, 'aria-haspopup': 'menu', 'aria-expanded': open, 'aria-controls': id })}

<MenuPanel bind:panel {id} {label} {width} data-placement={placement} data-match={match || undefined} {ontoggle} {onkeydown}>
  {@render children()}
</MenuPanel>
