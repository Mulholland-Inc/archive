export function supportsAnchor() {
  return typeof CSS !== 'undefined' && CSS.supports('position-area: bottom');
}

export function show(node, source) {
  try {
    node.showPopover({ source });
  } catch {
    try {
      node.showPopover();
    } catch {}
  }
}

export function hide(node) {
  try {
    node.hidePopover();
  } catch {}
}

export function focusables(root) {
  const owner = root.closest('[role="menu"],[role="listbox"]') ?? root;
  return [...root.querySelectorAll('[role="menuitem"]:not([aria-disabled="true"]),[role="menuitemcheckbox"]:not([aria-disabled="true"]),[role="menuitemradio"]:not([aria-disabled="true"]),[role="option"]:not([aria-disabled="true"])')].filter(
    (el) => el.parentElement.closest('[role="menu"],[role="listbox"]') === owner
  );
}

export function roving(event, root, { onclose } = {}) {
  const items = focusables(root);
  if (!items.length) return;
  const i = items.indexOf(document.activeElement);
  if (event.key === 'ArrowDown') {
    event.preventDefault();
    items[(i + 1) % items.length].focus();
  } else if (event.key === 'ArrowUp') {
    event.preventDefault();
    items[(i - 1 + items.length) % items.length].focus();
  } else if (event.key === 'Home') {
    event.preventDefault();
    items[0].focus();
  } else if (event.key === 'End') {
    event.preventDefault();
    items[items.length - 1].focus();
  } else if (event.key === 'Tab') {
    onclose?.();
  } else if (event.key.length === 1 && /\S/.test(event.key)) {
    const start = i + 1;
    const ordered = [...items.slice(start), ...items.slice(0, start)];
    const hit = ordered.find((el) => el.textContent.trim().toLowerCase().startsWith(event.key.toLowerCase()));
    hit?.focus();
  }
}
