let next = 1;

export const toasts = $state([]);

export function toast(message, { tone = 'neutral', action = null, timeout = 4000 } = {}) {
  const id = next++;
  toasts.push({ id, message, tone, action });
  if (timeout) setTimeout(() => dismiss(id), timeout);
  return id;
}

export function dismiss(id) {
  const i = toasts.findIndex((t) => t.id === id);
  if (i >= 0) toasts.splice(i, 1);
}
