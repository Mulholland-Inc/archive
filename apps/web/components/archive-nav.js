const links = [
  { href: './#features', label: 'How it works' },
  { href: 'pricing.html', label: 'Pricing' },
  { href: 'self-host.html', label: 'Self-host' },
  { href: 'https://docs.archive.us.org', label: 'Docs' },
];

class ArchiveNav extends HTMLElement {
  connectedCallback() {
    if (this.dataset.ready) return;
    this.dataset.ready = 'true';
    const here = location.pathname.split('/').pop() || 'index.html';
    const items = links
      .map(({ href, label }) => {
        const current = href === here ? ' aria-current="page"' : '';
        return `<li><a class="nav-link" href="${href}"${current}>${label}</a></li>`;
      })
      .join('');
    this.innerHTML = `
      <nav class="nav" aria-label="Main">
        <a class="nav-home" href="./" aria-label="Archive home"><archive-wordmark></archive-wordmark></a>
        <ul class="nav-links" id="nav-links">${items}</ul>
        <div class="nav-end">
          <a class="nav-link nav-quiet" href="self-host.html">Run it yourself</a>
          <a class="button button-primary button-sm" href="./#waitlist">Join the waitlist</a>
          <button class="nav-menu" type="button" aria-expanded="false" aria-controls="nav-links">
            <span class="visually-hidden">Menu</span><span class="nav-menu-bars" aria-hidden="true"></span>
          </button>
        </div>
      </nav>`;
    const menu = this.querySelector('.nav-menu');
    const nav = this.querySelector('.nav');
    menu.addEventListener('click', () => {
      const open = menu.getAttribute('aria-expanded') !== 'true';
      menu.setAttribute('aria-expanded', String(open));
      nav.classList.toggle('is-open', open);
    });
    this.addEventListener('keydown', (event) => {
      if (event.key !== 'Escape' || !nav.classList.contains('is-open')) return;
      menu.setAttribute('aria-expanded', 'false');
      nav.classList.remove('is-open');
      menu.focus();
    });
  }
}

customElements.define('archive-nav', ArchiveNav);
