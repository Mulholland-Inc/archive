import { openDialog } from './app-dialog.js';
import { label } from './ontology.js';

class FilterBar extends HTMLElement {
  show({ spec, fields, where }) {
    this.replaceChildren();
    for (const [key, value] of Object.entries(where)) {
      const f = fields.find((x) => x.key === key);
      if (!f) continue;
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'button button-secondary button-sm';
      const hidden = document.createElement('span');
      hidden.className = 'visually-hidden';
      hidden.textContent = 'Remove filter ';
      const v = document.createElement('field-value');
      v.setAttribute('plain', '');
      v.show(spec, { ...f.field, list: false }, f.field.type === 'boolean' ? value === 'true' : value);
      b.append(hidden, `${label(key)} is `, v, document.createElement('app-icon'));
      b.lastChild.setAttribute('name', 'close');
      b.addEventListener('click', () => {
        const next = { ...where };
        delete next[key];
        this.emit(next);
      });
      this.append(b);
    }
    const add = document.createElement('button');
    add.type = 'button';
    add.className = 'button button-secondary button-sm';
    add.innerHTML = '<app-icon name="plus"></app-icon><span>Filter</span>';
    add.addEventListener('click', () => this.add(spec, fields, where));
    this.append(add);
  }

  async add(spec, fields, where) {
    const usable = fields.filter((f) => !f.field.list);
    const body = document.createElement('div');
    body.innerHTML = '<label><span>Field</span><app-select data-field aria-label="Field"></app-select></label><label for="filter-value"><span>Value</span><field-input></field-input></label>';
    const select = body.querySelector('app-select');
    const input = body.querySelector('field-input');
    const pick = () => input.edit(spec, { ...usable.find((f) => f.key === select.value).field, required: true }, null, 'filter-value');
    select.addEventListener('change', pick);
    let value;
    const done = openDialog({
      title: 'Filter',
      body,
      actions: [
        { label: 'Cancel', value: '' },
        { label: 'Apply', value: 'ok', kind: 'primary' },
      ],
      onAction: () => {
        value = input.value;
        if (value == null) {
          input.error = ['Enter a value'];
          return false;
        }
      },
    });
    select.options = usable.map((f) => ({ value: f.key, label: label(f.key) }));
    select.value = usable[0].key;
    pick();
    if ((await done) === 'ok') this.emit({ ...where, [select.value]: String(value) });
  }

  emit(where) {
    this.dispatchEvent(new CustomEvent('filter', { detail: where, bubbles: true }));
  }
}

customElements.define('filter-bar', FilterBar);
