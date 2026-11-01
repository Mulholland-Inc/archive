export const SEPARATOR = '-';

export const contextMenu = $state({ items: [], x: 0, y: 0, keyboard: false, origin: null, at: 0 });

const resolve = (items) => (typeof items === 'function' ? items() : items) ?? [];

export function tidy(items) {
  const out = [];
  for (const item of items) {
    if (!item) continue;
    if (item === SEPARATOR && (!out.length || out.at(-1) === SEPARATOR)) continue;
    out.push(item);
  }
  if (out.at(-1) === SEPARATOR) out.pop();
  return out;
}

export function openContextMenu(items, { x, y, keyboard = false, origin = null }) {
  contextMenu.items = tidy(items);
  if (!contextMenu.items.length) return false;
  Object.assign(contextMenu, { x, y, keyboard, origin, at: Date.now() });
  return true;
}

export function closeContextMenu() {
  contextMenu.at = 0;
}

function native(event) {
  if (event.shiftKey && event.type === 'contextmenu') return true;
  if (event.target.closest?.('input, textarea, select, [contenteditable]:not([contenteditable="false"])')) return true;
  const sel = getSelection();
  return sel && !sel.isCollapsed && sel.containsNode(event.target, true);
}

export function contextmenu(items) {
  return (node) => {
    function open(event, keyboard) {
      if (event.defaultPrevented || native(event)) return;
      let x = event.clientX;
      let y = event.clientY;
      if (keyboard) {
        const r = event.target.getBoundingClientRect();
        x = r.left + Math.min(r.width / 2, 24);
        y = r.bottom;
      }
      if (!openContextMenu(resolve(items), { x, y, keyboard, origin: node })) return;
      event.preventDefault();
    }

    function oncontextmenu(event) {
      if (contextMenu.keyboard && Date.now() - contextMenu.at < 400) return event.preventDefault();
      open(event, event.button !== 2 && !event.clientX && !event.clientY);
    }

    function onkeydown(event) {
      if (event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10')) open(event, true);
    }

    node.addEventListener('contextmenu', oncontextmenu);
    node.addEventListener('keydown', onkeydown);
    return () => {
      node.removeEventListener('contextmenu', oncontextmenu);
      node.removeEventListener('keydown', onkeydown);
    };
  };
}
