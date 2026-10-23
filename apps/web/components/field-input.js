import { api } from './api.js';
import { endpoints } from './endpoints.js';
import { editText, kind } from './format.js';
import { recordTitle } from './ontology.js';

const modes = { email: 'email', phone: 'tel', url: 'url', money: 'decimal', decimal: 'decimal', number: 'decimal', integer: 'numeric', percentage: 'decimal' };

class FieldInput extends HTMLElement {
  edit(spec, field, value, id) {
    this.field = field;
    this.replaceChildren();
    const k = kind(spec, field);
    const none = field.required ? [] : [{ value: '', label: 'None' }];
    if (!field.list && ['enum', 'boolean', 'reference'].includes(k)) {
      const select = document.createElement('app-select');
      select.setAttribute('placeholder', 'Choose');
      if (id) select.setAttribute('aria-label', id);
      this.control = select;
      this.append(select);
      if (k === 'enum') select.options = [...none, ...field.enum.map((v) => ({ value: v, label: v }))];
      if (k === 'boolean') select.options = [...none, { value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }];
      select.value = value == null ? '' : String(value);
      if (k === 'reference') {
        select.options = value ? [{ value, label: 'Loading' }] : none;
        api(endpoints.records(field.to), { query: { limit: 1000 } }).then((rs) => {
          select.options = [...none, ...rs.map((r) => ({ value: r.id, label: recordTitle(spec, r) }))];
        });
      }
      this.read = () => (select.value === '' ? null : k === 'boolean' ? select.value === 'true' : select.value);
    } else {
      const input = document.createElement('input');
      input.type = 'text';
      input.autocomplete = 'off';
      if (id) input.id = id;
      if (modes[k]) input.inputMode = modes[k];
      input.value = editText(spec, field, value);
      this.control = input;
      this.append(input);
      this.read = () => {
        const text = input.value.trim();
        if (!text) return null;
        return field.list ? text.split(',').map((s) => s.trim()).filter(Boolean) : text;
      };
    }
    this.message = document.createElement('p');
    this.message.setAttribute('role', 'alert');
    this.message.hidden = true;
    this.append(this.message);
  }

  get value() {
    return this.read?.() ?? null;
  }

  set error(messages) {
    this.message.textContent = (messages ?? []).join(', ').replace(/^./, (c) => c.toUpperCase());
    this.message.hidden = !messages?.length;
    this.toggleAttribute('data-invalid', Boolean(messages?.length));
  }

  focus() {
    (this.control.button ?? this.control).focus();
    this.control.select?.();
  }
}

customElements.define('field-input', FieldInput);
