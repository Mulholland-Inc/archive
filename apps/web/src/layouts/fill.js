export function fill(min = 384) {
  return (node) => {
    const page = node.closest('.scroll');
    if (!page) return;
    const inner = page.firstElementChild;
    const size = () => {
      const pad = parseFloat(getComputedStyle(inner).paddingBlockEnd) || 0;
      const top = node.getBoundingClientRect().top - page.getBoundingClientRect().top + page.scrollTop;
      node.style.setProperty('--view-fill', `${Math.max(min, Math.floor(page.clientHeight - pad - top))}px`);
    };
    size();
    const ro = new ResizeObserver(size);
    ro.observe(page);
    ro.observe(inner);
    return () => ro.disconnect();
  };
}
