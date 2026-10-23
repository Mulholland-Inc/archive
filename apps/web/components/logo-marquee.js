class LogoMarquee extends HTMLElement {
  connectedCallback() {
    if (this.querySelector('.marquee-track')) return;
    const source = this.querySelector('ul');
    if (!source || !source.children.length) return;
    const originals = [...source.children];
    const wide = Math.max(this.clientWidth, window.screen?.width ?? 0);
    for (let round = 0; source.scrollWidth < wide && round < 8; round++) {
      for (const item of originals) {
        const extra = item.cloneNode(true);
        extra.setAttribute('aria-hidden', 'true');
        source.append(extra);
      }
    }
    const track = document.createElement('div');
    track.className = 'marquee-track';
    const copy = source.cloneNode(true);
    copy.removeAttribute('aria-label');
    copy.setAttribute('aria-hidden', 'true');
    source.replaceWith(track);
    track.append(source, copy);
    this.classList.add('is-moving');
  }
}

customElements.define('logo-marquee', LogoMarquee);
