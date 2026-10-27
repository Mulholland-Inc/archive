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
