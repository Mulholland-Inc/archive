<script>
  import { setContext, tick } from 'svelte';
  import MenuItems from './MenuItems.svelte';
  import MenuPanel from './MenuPanel.svelte';
  import { contextMenu, closeContextMenu } from '../lib/contextmenu.svelte.js';
  import { show, hide, roving, focusables } from '../lib/floating.js';

  let panel = $state();
  let left = $state(0);
  let top = $state(0);
  let origin = $state('top left');
  let opened = 0;
  let target = null;
  let back = null;

  const gap = 8;

  $effect(() => {
    if (!contextMenu.at) return hide(panel);
    place(contextMenu.x, contextMenu.y);
  });

  async function place(x, y) {
    target?.removeAttribute('data-contextmenu');
    target = contextMenu.origin;
    back = document.activeElement;
    left = x;
    top = y;
    await tick();
    opened = contextMenu.at;
    show(panel);
    const { width, height } = panel.getBoundingClientRect();
    const flipX = x + width > innerWidth - gap;
    const flipY = y + height > innerHeight - gap;
    left = Math.max(gap, flipX ? x - width : x);
    top = Math.max(gap, flipY ? Math.min(y - height, innerHeight - gap - height) : y);
    origin = `${flipY ? 'bottom' : 'top'} ${flipX ? 'right' : 'left'}`;
    target?.setAttribute('data-contextmenu', '');
    requestAnimationFrame(() => (contextMenu.keyboard ? focusables(panel)[0] : panel)?.focus());
  }

  function close() {
    hide(panel);
  }

  function ontoggle(event) {
    if (event.newState === 'open' || contextMenu.at !== opened) return;
    target?.removeAttribute('data-contextmenu');
    if (back?.isConnected && (panel.contains(document.activeElement) || document.activeElement === document.body)) back.focus({ preventScroll: true });
    target = back = null;
    closeContextMenu();
  }

  function onkeydown(event) {
    roving(event, panel, { onclose: close });
  }

  function dismiss(event) {
    if (contextMenu.at && !(event.target instanceof Node && panel.contains(event.target))) close();
  }

  function escape(event) {
    if (!contextMenu.at || event.key !== 'Escape') return;
    event.preventDefault();
    event.stopPropagation();
    close();
  }

  setContext('menu', { close });
</script>

<svelte:window onresize={dismiss} onblur={dismiss} onkeydowncapture={escape} />
<svelte:document onscrollcapture={dismiss} onpointerdowncapture={dismiss} />

<MenuPanel bind:panel kind="context" width="sm" popover="manual" label="Context menu" x={left} y={top} {origin} oncontextmenu={(event) => event.preventDefault()} {ontoggle} {onkeydown}>
  <MenuItems items={contextMenu.items} />
</MenuPanel>
