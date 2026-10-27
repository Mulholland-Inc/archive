<script>
  let { open = $bindable(false), onshift, side, children } = $props();
  const query = matchMedia('(max-width: 767px)');
  let mobile = $state(query.matches);
  let panel = $state();
  let drag = $state(null);
  let ready = $state(false);
  const moving = $derived(drag?.lock === 'x');
  const shown = $derived(moving ? drag.p : open ? 1 : 0);

  $effect(() => {
    if (mobile) onshift?.(shown * panel.offsetWidth, shown, !moving);
  });

  $effect(() => {
    const frame = requestAnimationFrame(() => (ready = true));
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
      cancelAnimationFrame(frame);
      query.removeEventListener('change', change);
      removeEventListener('touchstart', start, opts);
      removeEventListener('touchmove', move, opts);
      removeEventListener('touchend', end);
      removeEventListener('touchcancel', end);
    };
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<aside bind:this={panel} class="drawer" class:open class:ready class:moving inert={mobile && !open && !drag}>
  {@render side()}
</aside>
<div class="content" class:open class:ready class:moving style:--shown={shown}>
  {@render children()}
  <div class="shade" onclick={() => (open = false)} aria-hidden="true"></div>
</div>

<style>
  .drawer { position: sticky; top: var(--gutter); display: flex; flex-direction: column; align-self: start; height: calc(100vh - 2 * var(--gutter)); }
  .content { display: contents; }
  .shade { display: none; }
  @media (max-width: 767px) {
    .drawer { position: fixed; inset: 0 auto 0 0; width: var(--drawer); height: auto; padding: var(--space-2); visibility: hidden; }
    .drawer::before, .drawer::after { content: ''; flex: none; }
    .drawer::before { height: var(--safe-top); }
    .drawer::after { height: var(--safe-bottom); }
    .drawer.ready { transition: visibility 0s var(--dur-drawer); }
    .open, .moving { visibility: visible; }
    .drawer.ready.open, .drawer.ready.moving { transition: none; }
    .content {
      position: relative;
      z-index: 1;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      overflow: clip;
      background: var(--shell);
      border-radius: calc(var(--shown) * var(--radius-xl));
      box-shadow: var(--shadow);
      transform: translateX(calc(var(--shown) * var(--drawer)));
    }
    .content::before { content: ''; flex: none; height: var(--safe-top); }
    .content::after { content: ''; flex: none; height: min(max(var(--safe-bottom), var(--cover, 0px)), var(--cover-max)); }
    .content::before { position: sticky; top: 0; z-index: 2; background: var(--shell); }
    .content.ready { transition: transform var(--dur-drawer) var(--ease-drawer), border-radius var(--dur-drawer) var(--ease-drawer); }
    .shade {
      display: block;
      position: absolute;
      inset: 0;
      background: var(--backdrop);
      opacity: var(--shown);
      pointer-events: none;
      touch-action: none;
    }
    .content.ready .shade { transition: opacity var(--dur-drawer) var(--ease-drawer); }
    .open .shade { pointer-events: auto; }
    .content.moving, .content.moving .shade { transition: none; }
  }
</style>
