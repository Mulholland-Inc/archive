const vv = window.visualViewport;

if (vv) {
  const fit = () => {
    if (vv.scale > 1.01) return;
    document.documentElement.style.setProperty('--viewport-height', `${vv.height}px`);
    if (window.scrollY || vv.offsetTop) window.scrollTo(0, 0);
  };
  vv.addEventListener('resize', fit);
  vv.addEventListener('scroll', fit);
  fit();
}
