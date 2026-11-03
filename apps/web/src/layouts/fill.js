// fill is an attachment that sets --view-fill, which a layout's style takes as its height: as tall as the page
// has room for below its top, down to the page's own bottom padding, so the page does not scroll and the layout
// scrolls inside itself instead: a map's table, a board's lanes. It is never shorter than min pixels; below that
// the page scrolls as it would.
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
    // The page's size moves with the window, and what is above the layout with the header.
    const ro = new ResizeObserver(size);
    ro.observe(page);
    ro.observe(inner);
    return () => ro.disconnect();
  };
}
