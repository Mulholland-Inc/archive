const year = new Date().getFullYear();

const columns = [
  ['Product', [['How it works', './#features'], ['Pricing', 'pricing.html'], ['Join the waitlist', './#waitlist']]],
  ['Self-host', [['Install', 'self-host.html'], ['Run it anywhere', 'self-host.html#anywhere'], ['Docs', 'https://docs.archive.us.org']]],
  ['Company', [['About', './#private'], ['Contact', 'mailto:hello@archive.us.org']]],
  ['Legal', [['Privacy', './#private'], ['Status', 'https://status.archive.us.org']]],
];

class ArchiveFooter extends HTMLElement {
  connectedCallback() {
    if (this.dataset.ready) return;
    this.dataset.ready = 'true';
    const cols = columns
      .map(
        ([title, items]) => `
          <div class="footer-col">
            <h2 class="footer-title">${title}</h2>
            <ul>${items.map(([label, href]) => `<li><a href="${href}">${label}</a></li>`).join('')}</ul>
          </div>`,
      )
      .join('');
    this.innerHTML = `
      <footer class="footer">
        <div class="footer-cols">${cols}</div>
        <div class="footer-meta">
          <p>© ${year} Archive</p>
          <p class="footer-status"><span class="status-dot" aria-hidden="true"></span>Your data stays yours</p>
        </div>
        <div class="footer-mark" aria-hidden="true"><archive-wordmark></archive-wordmark></div>
      </footer>`;
  }
}

customElements.define('archive-footer', ArchiveFooter);
