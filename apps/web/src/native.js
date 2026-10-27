const cap = window.Capacitor;

export const native = !!cap?.isNativePlatform?.() && !!cap.PluginHeaders?.some((h) => h.name === 'Native');

export function call(method, options = {}) {
  return native ? cap.nativePromise('Native', method, options).catch(console.error) : Promise.resolve();
}

export function on(event, fn) {
  if (!native) return () => {};
  const l = cap.addListener('Native', event, fn);
  return () => l.remove();
}

if (native) cap.nativePromise('Keyboard', 'setAccessoryBarVisible', { isVisible: false }).catch(() => {});

if (native) {
  const editable = (el) => !!el?.closest?.('input, textarea, select, [contenteditable]:not([contenteditable="false"])');
  const control = (el) => !!el?.closest?.('a, button, label, summary, [role="button"], [role="option"], [role="textbox"], [popover]');
  const hide = () => {
    if (editable(document.activeElement)) document.activeElement.blur();
    call('dismiss');
  };
  let touch = null;
  addEventListener('touchstart', (e) => {
    const t = e.touches[0];
    touch = e.touches.length === 1 ? { x: t.clientX, y: t.clientY, target: e.target, moved: false, done: false } : null;
  }, { passive: true, capture: true });
  addEventListener('touchmove', (e) => {
    if (!touch) return;
    const t = e.touches[0];
    const dx = t.clientX - touch.x;
    const dy = t.clientY - touch.y;
    if (Math.hypot(dx, dy) > 8) touch.moved = true;
    if (!touch.done && !document.documentElement.classList.contains('flow') && dy > 48 && dy > Math.abs(dx) * 2 && !(editable(document.activeElement) && document.activeElement.contains(touch.target))) {
      touch.done = true;
      hide();
    }
  }, { passive: true, capture: true });
  addEventListener('touchend', () => {
    if (touch && !touch.moved && !editable(touch.target) && !control(touch.target)) hide();
    touch = null;
  }, { passive: true, capture: true });
}
