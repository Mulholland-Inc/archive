export const markSvg = `<svg class="wordmark-mark" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><rect x="5.5" y="2.5" width="13" height="2.5" rx="1.25"/><rect x="3.75" y="6.75" width="16.5" height="2.5" rx="1.25"/><rect x="2" y="11" width="20" height="10.5" rx="3"/></svg>`;

class ArchiveWordmark extends HTMLElement {
  connectedCallback() {
    if (this.dataset.ready) return;
    this.dataset.ready = 'true';
    this.innerHTML = `${markSvg}<span class="wordmark-text">Archive</span>`;
  }
}

customElements.define('archive-wordmark', ArchiveWordmark);
