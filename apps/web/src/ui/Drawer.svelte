<script>
  let { open = $bindable(false), children } = $props();
  const query = matchMedia('(max-width: 767px)');
  let mobile = $state(query.matches);
  let panel = $state();
  let drag = $state(null);
  const shown = $derived(drag?.lock === 'x' ? drag.p : open ? 1 : 0);

  $effect(() => {
    const change = () => (mobile = query.matches);
    query.addEventListener('change', change);
    const start = (e) => {
      const t = e.touches[0];
      if (!mobile || e.touches.length > 1 || (!open && t.clientX > panel.offsetWidth / 10)) return;
      drag = { x: t.clientX, y: t.clientY, from: open ? 1 : 0, p: open ? 1 : 0, dx: 0, at: e.timeStamp, v: 0, lock: null, w: panel.offsetWidth };
    };
    const move = (e) => {
      if (!drag) return;
      const t = e.touches[0];
      const dx = t.clientX - drag.x;
      const dy = t.clientY - drag.y;
      if (!drag.lock) {
        if (Math.hypot(dx, dy) < 8) return;
        drag.lock = Math.abs(dx) > Math.abs(dy) ? 'x' : 'y';
      }
      if (drag.lock === 'y') return (drag = null);
      e.preventDefault();
      drag.v = (dx - drag.dx) / Math.max(1, e.timeStamp - drag.at);
      drag.dx = dx;
      drag.at = e.timeStamp;
      drag.p = Math.min(1, Math.max(0, drag.from + dx / drag.w));
    };
    const end = () => {
      if (drag?.lock === 'x') open = Math.abs(drag.v) > 0.3 ? drag.v > 0 : drag.p > 0.5;
      drag = null;
    };
    const opts = { passive: false };
    addEventListener('touchstart', start, opts);
    addEventListener('touchmove', move, opts);
    addEventListener('touchend', end);
    addEventListener('touchcancel', end);
    return () => {
      query.removeEventListener('change', change);
      removeEventListener('touchstart', start, opts);
      removeEventListener('touchmove', move, opts);
      removeEventListener('touchend', end);
      removeEventListener('touchcancel', end);
    };
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<aside
  bind:this={panel}
  class="drawer"
  class:open
  class:dragging={drag?.lock === 'x'}
  inert={mobile && !open && !drag}
  style:--shown={shown}
>
  {@render children()}
</aside>
<div class="scrim" class:open class:dragging={drag?.lock === 'x'} style:--shown={shown} onclick={() => (open = false)} aria-hidden="true"></div>

<style>
  .drawer { display: flex; flex-direction: column; min-height: 0; padding: var(--space-2); }
  .scrim { display: none; }
  @media (max-width: 767px) {
    .drawer {
      position: fixed;
      inset: 0 auto 0 0;
      z-index: 2;
      width: var(--drawer);
      background: var(--shell);
      box-shadow: var(--shadow);
      transform: translateX(calc((var(--shown) - 1) * 100%));
      visibility: hidden;
      touch-action: pan-y;
      transition: transform var(--dur-drawer) var(--ease-drawer), visibility 0s var(--dur-drawer);
    }
    .drawer::before, .drawer::after { content: ''; flex: none; }
    .drawer::before { height: var(--safe-top); }
    .drawer::after { height: var(--safe-bottom); }
    .open, .dragging { visibility: visible; transition: transform var(--dur-drawer) var(--ease-drawer); }
    .scrim {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 1;
      background: var(--backdrop);
      opacity: var(--shown);
      pointer-events: none;
      touch-action: none;
      transition: opacity var(--dur-drawer) var(--ease-drawer);
    }
    .scrim.open { pointer-events: auto; }
    .dragging { transition: none; }
  }
</style>
