export type Toast = { id: number; message: string; tone: 'info' | 'success' | 'danger'; action?: { label: string; run: () => void } };

export const toasts = $state<Toast[]>([]);
let next = 1;

export function toast(message: string, tone: Toast['tone'] = 'info', action?: Toast['action']) {
  const id = next++;
  toasts.push({ id, message, tone, action });
  setTimeout(() => dismiss(id), tone === 'danger' ? 7000 : 4000);
}

export function dismiss(id: number) {
  const i = toasts.findIndex((t) => t.id === id);
  if (i >= 0) toasts.splice(i, 1);
}

export function fail(e: unknown) {
  toast(e instanceof Error ? e.message : String(e), 'danger');
}
