const paths = {
  search: '<circle cx="7" cy="7" r="4.5"/><path d="m10.5 10.5 3 3"/>',
  menu: '<path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h11"/>',
  close: '<path d="m4 4 8 8M12 4l-8 8"/>',
  plus: '<path d="M8 3v10M3 8h10"/>',
  record: '<rect x="3" y="2.5" width="10" height="11" rx="2"/><path d="M5.5 6h5M5.5 8.5h5M5.5 11h3"/>',
  changes: '<path d="M2.5 8a5.5 5.5 0 1 0 1.6-3.9"/><path d="M2.5 2.5v2.5H5"/><path d="M8 5v3l2 1.5"/>',
  settings: '<path d="M2.5 4.5h6M11.5 4.5h2M2.5 11.5h2M7.5 11.5h6"/><circle cx="10" cy="4.5" r="1.5"/><circle cx="6" cy="11.5" r="1.5"/>',
  left: '<path d="m10 3.5-4.5 4.5 4.5 4.5"/>',
  right: '<path d="m6 3.5 4.5 4.5-4.5 4.5"/>',
  down: '<path d="m4 6 4 4 4-4"/>',
  check: '<path d="m3.5 8.5 3 3 6-7"/>',
  edit: '<path d="M10.5 2.5 13.5 5.5 6 13H3v-3z"/><path d="m9 4 3 3"/>',
  rename: '<path d="M3.5 4h7M7 4v8.5M12.5 3v10"/>',
  trash: '<path d="M3 4.5h10M6.5 4.5V3h3v1.5"/><path d="M4.5 4.5 5 13.5h6l.5-9"/>',
  table: '<rect x="2.5" y="3" width="11" height="10" rx="2"/><path d="M2.5 6.5h11M6.5 6.5V13"/>',
  board: '<rect x="2.5" y="3" width="3" height="10" rx="1"/><rect x="6.5" y="3" width="3" height="6" rx="1"/><rect x="10.5" y="3" width="3" height="8" rx="1"/>',
  calendar: '<rect x="2.5" y="3.5" width="11" height="10" rx="2"/><path d="M2.5 7h11M5.5 2v3M10.5 2v3"/>',
  types: '<rect x="2.5" y="2.5" width="4.5" height="4.5" rx="1.25"/><rect x="9" y="2.5" width="4.5" height="4.5" rx="1.25"/><rect x="2.5" y="9" width="4.5" height="4.5" rx="1.25"/><rect x="9" y="9" width="4.5" height="4.5" rx="1.25"/>',
  key: '<circle cx="5.5" cy="10.5" r="3"/><path d="m7.6 8.4 5.4-5.4M11 5l1.5 1.5"/>',
  mail: '<rect x="2" y="3.5" width="12" height="9" rx="2"/><path d="m2.5 4.5 5.5 4 5.5-4"/>',
  file: '<path d="M4 2.5h5l3 3v8H4z"/><path d="M9 2.5v3h3"/>',
  upload: '<path d="M8 10.5V3M5 5.5 8 2.5l3 3"/><path d="M3 10.5v2h10v-2"/>',
  download: '<path d="M8 2.5V10M5 7l3 3 3-3"/><path d="M3 10.5v2h10v-2"/>',
  source: '<path d="M3 4.5c0-1.1 2.2-2 5-2s5 .9 5 2-2.2 2-5 2-5-.9-5-2z"/><path d="M3 4.5v7c0 1.1 2.2 2 5 2s5-.9 5-2v-7M3 8c0 1.1 2.2 2 5 2s5-.9 5-2"/>',
  link: '<path d="M7 9a2.5 2.5 0 0 0 3.5 0l2-2A2.5 2.5 0 0 0 9 3.5l-.5.5"/><path d="M9 7a2.5 2.5 0 0 0-3.5 0l-2 2A2.5 2.5 0 0 0 7 12.5l.5-.5"/>',
  gallery: '<rect x="2.5" y="3" width="11" height="10" rx="2"/><circle cx="6" cy="6.5" r="1"/><path d="m2.5 11.5 3.5-3 2.5 2 2-1.5 3 2.5"/>',
  play: '<path d="M5 3.5v9l7-4.5z"/>',
  pause: '<path d="M5.5 3.5v9M10.5 3.5v9"/>',
  view: '<path d="M4 8h8M4 5h8M4 11h5"/>',
  space: '<circle cx="6" cy="6" r="2.5"/><path d="M2 13c.5-2.2 2-3.5 4-3.5s3.5 1.3 4 3.5"/><path d="M10.5 3.6a2.5 2.5 0 0 1 0 4.8M12 9.8c1 .6 1.7 1.7 2 3.2"/>',
  google: '<path class="fill" d="M15.5 8.2c0-.6-.1-1.1-.2-1.6H8v3h4.2a3.6 3.6 0 0 1-1.6 2.4v2h2.6c1.5-1.4 2.3-3.4 2.3-5.8zM8 16c2.2 0 4-.7 5.3-2l-2.6-2c-.7.5-1.6.8-2.7.8-2.1 0-3.9-1.4-4.5-3.3H.8v2.1A8 8 0 0 0 8 16zM3.5 9.5a4.8 4.8 0 0 1 0-3V4.4H.8a8 8 0 0 0 0 7.2zM8 3.2c1.2 0 2.2.4 3.1 1.2l2.3-2.3A8 8 0 0 0 .8 4.4l2.7 2.1C4.1 4.6 5.9 3.2 8 3.2z"/>',
  microsoft: '<path class="fill" d="M1 1h6.5v6.5H1zM8.5 1H15v6.5H8.5zM1 8.5h6.5V15H1zM8.5 8.5H15V15H8.5z"/>',
};

class AppIcon extends HTMLElement {
  static observedAttributes = ['name'];

  attributeChangedCallback() {
    this.render();
  }

  connectedCallback() {
    this.setAttribute('aria-hidden', 'true');
    this.render();
  }

  render() {
    this.innerHTML = `<svg viewBox="0 0 16 16" focusable="false">${paths[this.getAttribute('name')] ?? ''}</svg>`;
  }
}

customElements.define('app-icon', AppIcon);
