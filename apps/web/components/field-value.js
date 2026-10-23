import { pages } from './endpoints.js';
import { formatValue, kind } from './format.js';
import { loadRecord, recordTitle } from './ontology.js';

const hrefs = {
  email: (v) => `mailto:${v}`,
  phone: (v) => `tel:${v}`,
  url: (v) => v,
};

class FieldValue extends HTMLElement {
  show(spec, field, value) {
    this.replaceChildren();
    if (value == null || value === '' || (Array.isArray(value) && !value.length)) {
      this.textContent = this.getAttribute('empty') ?? '';
      this.toggleAttribute('data-empty', true);
      return;
    }
    this.removeAttribute('data-empty');
    const k = kind(spec, field);
    const values = Array.isArray(value) ? value : [value];
    const parts = values.map((v) => {
      const plain = this.hasAttribute('plain');
      if (k === 'reference') {
        const a = document.createElement(plain ? 'span' : 'a');
        if (!plain) a.href = pages.record(field.to, v);
        a.textContent = 'Loading';
        loadRecord(field.to, v).then((r) => (a.textContent = r ? recordTitle(spec, r) : 'Missing record'));
        return a;
      }
      if (k === 'file' && !plain && v?.sha256) {
        const a = document.createElement('a');
        a.href = pages.file({ sha256: v.sha256 });
        a.textContent = v.name;
        return a;
      }
      const text = formatValue(spec, { ...field, list: false }, v);
      if (!hrefs[k] || plain) return document.createTextNode(text);
      const a = document.createElement('a');
      a.href = hrefs[k](v);
      a.textContent = text;
      if (k === 'url') a.target = '_blank';
      if (k === 'url') a.rel = 'noopener';
      return a;
    });
    parts.forEach((p, i) => this.append(...(i ? [', ', p] : [p])));
  }
}

customElements.define('field-value', FieldValue);
