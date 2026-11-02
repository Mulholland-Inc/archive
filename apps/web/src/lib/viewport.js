// iOS opens the keyboard over the page and pans it, rather than resizing it.
// Size the app to the visible area instead and undo the pan, so the composer
// sits just above the keyboard and the top bar stays put.
const vv = window.visualViewport;

if (vv) {
  const fit = () => {
    // A pinch-zoom shrinks the visible area too; leave that one alone.
    if (vv.scale > 1.01) return;
    document.documentElement.style.setProperty('--viewport-height', `${vv.height}px`);
    if (window.scrollY || vv.offsetTop) window.scrollTo(0, 0);
  };
  vv.addEventListener('resize', fit);
  vv.addEventListener('scroll', fit);
  fit();
}
